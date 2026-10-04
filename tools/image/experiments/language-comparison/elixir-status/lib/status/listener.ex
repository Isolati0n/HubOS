defmodule Status.Listener do
  @moduledoc """
  Accepts TCP connections and hands each one to its own short-lived process.

  The HTTP parsing is done by the Erlang VM itself (`packet: :http_bin`), so there is no hand-written parser.
  Settings come from environment variables (read when the program starts):

    * `PORT` (default 8481), `HUBOS_LISTEN_ADDR` (default 127.0.0.1)
    * `HUBOS_RELEASE_FILE` (default /etc/hubos-release), `HUBOS_CMDLINE_FILE` (default /proc/cmdline)
    * `HUBOS_TEST_CRASH=1` turns on the /v1/crash test endpoints (experiment only)
  """
  use Task, restart: :permanent

  def start_link(_arg), do: Task.start_link(__MODULE__, :run, [])

  def run do
    port = String.to_integer(System.get_env("PORT", "8481"))
    {:ok, addr} = System.get_env("HUBOS_LISTEN_ADDR", "127.0.0.1") |> String.to_charlist() |> :inet.parse_address()

    {:ok, lsock} =
      :gen_tcp.listen(port, [:binary, packet: :http_bin, active: false, reuseaddr: true, ip: addr, backlog: 128])

    require Logger
    Logger.info("status: listening on #{:inet.ntoa(addr)}:#{port}")
    accept_loop(lsock)
  end

  defp accept_loop(lsock) do
    case :gen_tcp.accept(lsock) do
      {:ok, sock} ->
        {:ok, pid} = Task.Supervisor.start_child(Status.Handlers, Status.Handler, :serve, [sock])
        :ok = :gen_tcp.controlling_process(sock, pid)
        send(pid, :go)
        accept_loop(lsock)

      {:error, _} ->
        accept_loop(lsock)
    end
  end
end
