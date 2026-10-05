# PROTOTYPE of the "policy brain" in Elixir/OTP (bench only, never for the image). Same behaviour as the Go
# prototype in ../go: escalation ladder, health probe, status socket, crash record, alert file, hold-and-start of
# dependents. It commands s6 (architecture C). The OTP part is the shape: one supervised GenServer per service, a
# Supervisor with restart intensity above them, and the BEAM itself supervised by s6 (so a bug in the brain, in one
# watcher or in the whole VM never touches the managed services). Distribution is off (no -name, no epmd, no cookie).
defmodule Brain.Ladder do
  # Pure ladder: no I/O, so it can be tested without a machine. Times are monotonic milliseconds.
  @window 60_000
  @backoff_after 2
  @deps_after 4
  @give_up_after 6
  @pause 1_000
  defstruct fails: [], step: :running, until: 0, restarts: 0, last_fail: nil, cause: ""

  def fail(%__MODULE__{} = l, now, cause) do
    fails = [now | Enum.filter(l.fails, &(now - &1 < @window))]
    n = length(fails)
    l = %{l | fails: fails, restarts: l.restarts + 1, last_fail: now, cause: cause}

    cond do
      l.step == :degraded -> {l, :nothing}
      n >= @give_up_after -> {%{l | step: :degraded}, :give_up}
      n >= @deps_after -> {%{l | step: :deps_hit, until: now + pause(n)}, :restart_deps}
      n >= @backoff_after -> {%{l | step: :backoff, until: now + pause(n)}, :stop_for_backoff}
      true -> {l, :nothing}
    end
  end

  defp pause(n), do: min(@pause * Integer.pow(2, max(n - @backoff_after, 0)), 60_000)

  def tick(%__MODULE__{step: s, until: u} = l, now) when s in [:backoff, :deps_hit] and now >= u,
    do: {%{l | step: :running}, true}

  def tick(%__MODULE__{step: :running, fails: [last | _]} = l, now) when now - last > @window,
    do: {%{l | fails: []}, false}

  def tick(l, _now), do: {l, false}
  def retry(l), do: %{l | fails: [], step: :running}
end

defmodule Brain.S6 do
  @scan "/run/service"
  def run(opt, name), do: System.cmd("s6-svc", [opt, "#{@scan}/#{name}"], stderr_to_stdout: true)

  def status(name) do
    case System.cmd("s6-svstat", ["#{@scan}/#{name}"], stderr_to_stdout: true) do
      {out, 0} ->
        case Regex.run(~r/^up \(pid (\d+)/, out) do
          [_, pid] -> {true, String.to_integer(pid), String.contains?(out, ", ready "), ""}
          _ -> {false, 0, false, out |> String.trim()}
        end

      _ ->
        {false, 0, false, "no supervisor"}
    end
  end

  def start(n), do: run("-u", n)
  def stop(n), do: run("-d", n)
  def kill(n), do: run("-k", n)
  def restart(n), do: run("-r", n)
end

defmodule Brain.Probe do
  # Ask the service on its unix socket; wants an answer that starts with "ok" within a second.
  def ok?(sock, request) do
    with {:ok, c} <- :gen_tcp.connect({:local, String.to_charlist(sock)}, 0, [:binary, active: false, packet: :line], 1000),
         :ok <- :gen_tcp.send(c, request <> "\n"),
         {:ok, line} <- :gen_tcp.recv(c, 0, 1000) do
      :gen_tcp.close(c)
      String.starts_with?(line, "ok")
    else
      _ -> false
    end
  end
end

defmodule Brain.Watcher do
  use GenServer
  alias Brain.{Ladder, S6, Probe}

  @tick 1_000
  def start_link(spec), do: GenServer.start_link(__MODULE__, spec, name: via(spec.name))
  def via(name), do: String.to_atom("w_" <> name)
  def child_spec(spec), do: %{id: spec.name, start: {__MODULE__, :start_link, [spec]}, restart: :permanent}
  def now, do: System.monotonic_time(:millisecond)

  @impl true
  def init(spec) do
    st = Map.merge(spec, %{l: %Ladder{}, last_up: false, expect_down: false, held: false, pid: 0, started: 0, bad: 0})
    # insert_new: a restarted watcher must not wipe what the others already know about this service
    :ets.insert_new(:brain, {spec.name, false, false, 0, 0})
    Process.send_after(self(), :tick, @tick)
    {:ok, st}
  end

  @impl true
  def handle_call(:status, _f, s) do
    ago = if s.l.last_fail, do: "#{div(now() - s.l.last_fail, 1000)}s", else: "never"
    {:reply, "#{s.name} state=#{s.l.step} up=#{s.last_up} pid=#{s.pid} restarts=#{s.l.restarts} last_failure=#{ago} needs=#{inspect(s.needs)} cause=#{inspect(s.l.cause)}\n", s}
  end

  def handle_call(:retry, _f, s) do
    S6.start(s.name)
    {:reply, :ok, %{s | l: Ladder.retry(s.l), expect_down: false}}
  end

  @impl true
  def handle_info(:tick, s) do
    s = step(s, now())
    Process.send_after(self(), :tick, @tick)
    {:noreply, s}
  end

  @impl true
  # TEST HOOK: a bug in the policy code of one service's watcher. OTP ends only this process and restarts it.
  def handle_cast(:boom, _s), do: raise("test hook: bug in the policy code")

  defp needs_ok?(s), do: Enum.all?(s.needs, fn n -> match?([{_, _, true, _, _}], :ets.lookup(:brain, n)) end)

  defp step(s, now) do
    {up, pid, ready, last} = S6.status(s.name)
    s =
      if up and pid != s.pid do
        s = if s.pid != 0 and s.last_up and not s.held, do: fail(s, now, "restarted unseen"), else: s
        %{s | pid: pid, started: now}
      else
        s
      end

    :ets.insert(:brain, {s.name, up, ready, pid, s.started})

    cond do
      not needs_ok?(s) ->
        if up, do: S6.stop(s.name)
        %{s | held: true, expect_down: true, last_up: up}

      s.held ->
        if s.l.step == :running, do: S6.start(s.name)
        %{s | held: false, expect_down: false, last_up: up}

      true ->
        s = if not up and s.last_up and not s.expect_down, do: fail(s, now, last), else: s
        s = %{s | last_up: up}
        s = probe(s, up, now)
        {l, start?} = Ladder.tick(s.l, now)
        if start?, do: S6.start(s.name)
        %{s | l: l, expect_down: if(start?, do: false, else: s.expect_down)}
    end
  end

  defp probe(%{probe: nil} = s, _, _), do: s
  defp probe(s, false, _), do: s
  defp probe(s, true, now) do
    if now - s.started < 8_000 or Probe.ok?(s.probe, "state") do
      %{s | bad: 0}
    else
      if s.bad + 1 >= 3 do
        s = fail(%{s | bad: 0}, now, "health probe: no answer")
        S6.kill(s.name)
        s
      else
        %{s | bad: s.bad + 1}
      end
    end
  end

  defp fail(s, now, cause) do
    {l, action} = Ladder.fail(s.l, now, cause)
    crash_record(s, cause)
    IO.puts("brain: #{s.name} failed (#{cause}), action=#{action}")
    s = %{s | l: l}

    case action do
      :stop_for_backoff -> S6.stop(s.name); %{s | expect_down: true}
      :restart_deps ->
        S6.stop(s.name)
        for d <- s.dependents, do: S6.restart(d)
        %{s | expect_down: true}
      :give_up ->
        S6.stop(s.name)
        File.write("/run/hubos/alert", "#{s.name} degraded: #{cause} (retrying stopped; the owner must act)\n", [:append])
        %{s | expect_down: true}
      :nothing -> s
    end
  end

  defp crash_record(s, cause) do
    File.mkdir_p!("/var/log/crash")
    tail = case File.read("/var/log/#{s.name}/current") do
      {:ok, b} -> b |> String.trim() |> String.split("\n") |> Enum.take(-8) |> Enum.join("\n")
      _ -> ""
    end
    n = rem(System.unique_integer([:positive]), 20)
    File.write("/var/log/crash/#{s.name}-#{n}.txt", "service=#{s.name} cause=#{inspect(cause)} failures=#{length(s.l.fails)}\n#{tail}\n")
  end
end

defmodule Brain.Status do
  # status socket: "status" | "retry NAME"; one process per connection under a Task.Supervisor.
  def start_link(path) do
    File.rm(path)
    {:ok, l} = :gen_tcp.listen(0, [:local, {:ifaddr, {:local, String.to_charlist(path)}}, :binary, active: false, packet: :line, reuseaddr: true])
    {:ok, spawn_link(fn -> accept(l) end)}
  end

  def child_spec(path), do: %{id: __MODULE__, start: {__MODULE__, :start_link, [path]}, restart: :permanent}

  defp accept(l) do
    {:ok, c} = :gen_tcp.accept(l)
    pid = spawn(fn -> serve(c) end)
    :gen_tcp.controlling_process(c, pid)
    accept(l)
  end

  defp serve(c) do
    {:ok, line} = :gen_tcp.recv(c, 0, 2000)
    reply =
      case String.split(String.trim(line)) do
        ["status"] -> Enum.map_join(Brain.services(), fn n -> GenServer.call(Brain.Watcher.via(n), :status) end)
        ["retry", n] -> if n in Brain.services(), do: (GenServer.call(Brain.Watcher.via(n), :retry); File.rm("/run/hubos/alert"); "ok\n"), else: "error: unknown service\n"
        ["crashtest", n] -> GenServer.cast(Brain.Watcher.via(n), :boom); "ok (raising)\n"
        _ -> "error: status | retry NAME\n"
      end
    :gen_tcp.send(c, reply)
    :gen_tcp.close(c)
  end
end

defmodule Brain do
  @graph [
    {"seatd", [], nil}, {"udevd", [], nil}, {"dbus", [], nil},
    {"driftwm", ["seatd", "udevd"], "/run/dw/wayland-1"},
    {"waybar", ["driftwm", "dbus"], nil}, {"hubd", ["driftwm"], nil}
  ]
  def services, do: Enum.map(@graph, &elem(&1, 0))

  def main do
    :ets.new(:brain, [:named_table, :public, :set])
    File.mkdir_p!("/run/hubos")
    specs =
      for {n, needs, probe} <- @graph do
        %{name: n, needs: needs, probe: probe, dependents: for({m, ns, _} <- @graph, n in ns, do: m)}
      end
    children = Enum.map(specs, &Brain.Watcher.child_spec/1) ++ [Brain.Status.child_spec("/run/hubos/brain.sock")]
    # restart intensity: more than 10 crashes of the policy code in 10 s ends the whole brain, which s6 restarts
    {:ok, _} = Supervisor.start_link(children, strategy: :one_for_one, max_restarts: 10, max_seconds: 10)
    IO.puts("brain(elixir): started")
    Process.sleep(:infinity)
  end
end
