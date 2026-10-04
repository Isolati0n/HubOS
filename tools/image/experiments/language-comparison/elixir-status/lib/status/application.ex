defmodule Status.Application do
  @moduledoc false
  use Application

  @impl true
  def start(_type, _args) do
    children = [
      {Task.Supervisor, name: Status.Handlers},
      Status.Listener
    ]

    Supervisor.start_link(children, strategy: :one_for_one, name: Status.Supervisor)
  end
end
