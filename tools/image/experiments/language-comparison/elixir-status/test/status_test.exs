defmodule StatusTest do
  use ExUnit.Case

  test "release and flavor" do
    assert Status.Handler.value("version=7\nflavor=hub\n", "version=") == "7"
    assert Status.Handler.value("version=7\nflavor=hub\n", "flavor=") == "hub"
    assert Status.Handler.value("", "version=") == "unknown"
  end

  test "slot" do
    assert Status.Handler.slot("console=ttyS0 hubos.slot=b ro\n") == "b"
    assert Status.Handler.slot("console=ttyS0 ro") == "unknown"
    assert Status.Handler.slot("hubos.slot=c") == "unknown"
    assert Status.Handler.slot("hubos.slot=a hubos.slot=b") == "b"
    assert Status.Handler.slot("xhubos.slot=a") == "unknown"
  end

  test "the endpoint answers over a real socket" do
    {:ok, s} = :gen_tcp.connect(~c"127.0.0.1", 8481, [:binary, active: false])
    :gen_tcp.send(s, "GET /v1/status HTTP/1.1\r\nHost: x\r\n\r\n")
    {:ok, data} = :gen_tcp.recv(s, 0, 2000)
    assert data =~ "200 OK"
    assert data =~ ~s("slot":")
  end
end
