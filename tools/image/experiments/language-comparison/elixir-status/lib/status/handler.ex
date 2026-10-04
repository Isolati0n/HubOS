defmodule Status.Handler do
  @moduledoc """
  One process per connection. If it crashes, only this connection is lost: the supervisor and the listener go on.
  """
  @max_headers 100
  @timeout 5_000

  def serve(sock) do
    receive do
      :go -> :ok
    after
      @timeout -> :gen_tcp.close(sock)
    end

    {code, body} = request(sock)
    reply(sock, code, body)
    :gen_tcp.close(sock)
  end

  defp request(sock) do
    case :gen_tcp.recv(sock, 0, @timeout) do
      {:ok, {:http_request, method, {:abs_path, path}, _vsn}} ->
        skip_headers(sock, @max_headers)
        route(method, path)

      _ ->
        {400, ~s({"error":"bad request"})}
    end
  end

  defp skip_headers(_sock, 0), do: :ok

  defp skip_headers(sock, n) do
    case :gen_tcp.recv(sock, 0, @timeout) do
      {:ok, :http_eoh} -> :ok
      {:ok, _header} -> skip_headers(sock, n - 1)
      _ -> :ok
    end
  end

  defp route(:GET, "/v1/status"), do: {200, status_json()}

  defp route(_method, "/v1/crash" = _path) do
    if System.get_env("HUBOS_TEST_CRASH") == "1", do: raise("test crash in a request handler")
    not_found()
  end

  defp route(_method, "/v1/crash-process") do
    # A crash in a process the handler started (unlinked) does not touch the handler or anything else.
    if System.get_env("HUBOS_TEST_CRASH") == "1" do
      spawn(fn -> raise "test crash in a process started by a handler" end)
      Process.sleep(1000)
      {200, ~s({"note":"survived"})}
    else
      not_found()
    end
  end

  defp route(_method, _path), do: not_found()

  defp not_found, do: {404, ~s({"error":"no such endpoint"})}

  defp reply(sock, code, body) do
    text = if code == 200, do: "OK", else: if(code == 404, do: "Not Found", else: "Bad Request")

    :gen_tcp.send(sock, [
      "HTTP/1.1 #{code} #{text}\r\nContent-Type: application/json\r\nContent-Length: ",
      Integer.to_string(byte_size(body)),
      "\r\nConnection: close\r\n\r\n",
      body
    ])
  end

  # ---- the status answer: same fields and same bytes as the Go version ----

  def status_json do
    release = File.read(System.get_env("HUBOS_RELEASE_FILE", "/etc/hubos-release")) |> text()
    cmdline = File.read(System.get_env("HUBOS_CMDLINE_FILE", "/proc/cmdline")) |> text()

    ~s({"release":#{json(value(release, "version="))},"flavor":#{json(value(release, "flavor="))},"slot":#{json(slot(cmdline))}})
  end

  defp text({:ok, t}), do: t
  defp text(_), do: ""

  @doc false
  def value(text, prefix) do
    text
    |> String.split("\n")
    |> Enum.reduce("unknown", fn line, acc ->
      if String.starts_with?(line, prefix) do
        line |> binary_part(byte_size(prefix), byte_size(line) - byte_size(prefix)) |> String.trim()
      else
        acc
      end
    end)
  end

  @doc false
  def slot(cmdline) do
    cmdline
    |> String.split()
    |> Enum.reduce("unknown", fn
      "hubos.slot=" <> v, _acc when v in ["a", "b"] -> v
      "hubos.slot=" <> _, _acc -> "unknown"
      _, acc -> acc
    end)
  end

  # JSON string by hand (OTP 25 has no :json module; that came with OTP 27).
  defp json(s) do
    ["\"", for(<<c <- s>>, into: "", do: esc(c)), "\""] |> IO.iodata_to_binary()
  end

  defp esc(?"), do: "\\\""
  defp esc(?\\), do: "\\\\"
  defp esc(c) when c < 0x20, do: "\\u" <> String.pad_leading(Integer.to_string(c, 16), 4, "0")
  defp esc(c), do: <<c>>
end
