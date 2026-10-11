#!/usr/bin/env escript
%% Receives a file descriptor over a Unix SOCK_SEQPACKET socket (OTP 'socket' module, control message 'rights'),
%% turns it into a socket and writes a line to it. Usage: escript receiver.escript SOCKET_PATH
%% TEST CODE for docs/research (see tools/fdpass/fdpass_test.go). Not part of hubd.
main([Path]) ->
    file:delete(Path),
    {ok, L} = socket:open(local, seqpacket, default),
    ok = socket:bind(L, #{family => local, path => Path}),
    ok = socket:listen(L),
    io:format("READY~n"),
    {ok, A} = socket:accept(L, 20000),
    {ok, #{iov := [Data], ctrl := Ctrl}} = socket:recvmsg(A, 64, 64, [], 20000),
    io:format("GOT-DATA ~s~n", [Data]),
    [Fd] = [F || #{level := socket, type := rights, data := Bin} <- Ctrl, <<F:32/native-signed>> <= Bin],
    io:format("GOT-FD ~p~n", [Fd]),
    {ok, S} = socket:open(Fd, #{domain => local, type => stream, protocol => default}),
    ok = socket:send(S, <<"hello over the passed fd\n">>),
    ok = socket:close(S),
    ok = socket:close(A),
    ok = socket:close(L),
    io:format("DONE~n").
