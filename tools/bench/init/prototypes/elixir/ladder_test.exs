Code.require_file("brain.ex", __DIR__)
ExUnit.start()

defmodule LadderTest do
  use ExUnit.Case
  alias Brain.Ladder

  test "the ladder ends in an alert, never a reboot, and a degraded service is not started by itself" do
    {actions, l} =
      Enum.map_reduce(1..8, %Ladder{}, fn i, l ->
        {l, a} = Ladder.fail(l, i * 1000, "exit 1")
        {a, l}
      end)

    assert actions == [:nothing, :stop_for_backoff, :stop_for_backoff, :restart_deps, :restart_deps, :give_up, :nothing, :nothing]
    assert l.step == :degraded
    assert {_, false} = Ladder.tick(l, 3_600_000)
    assert Ladder.retry(l).step == :running
  end

  test "pause grows and a quiet window forgives" do
    {l, _} = Ladder.fail(%Ladder{}, 0, "x")
    {l, :stop_for_backoff} = Ladder.fail(l, 0, "x")
    assert l.until == 1_000
    assert {l, true} = Ladder.tick(l, 1_000)
    {l, _} = Ladder.fail(l, 1_000, "x")
    assert l.until == 1_000 + 2_000
    assert {%{fails: []}, false} = Ladder.tick(%{l | step: :running}, 1_000 + 61_000)
  end
end
