defmodule Status.MixProject do
  use Mix.Project

  # EXPERIMENT (docs/proposals/language-comparison.md): the recovery agent's status endpoint in Elixir.
  # No dependencies at all (no Hex): only Elixir and Erlang/OTP themselves.
  def project do
    [
      app: :status,
      version: "0.1.0",
      elixir: "~> 1.14",
      deps: [],
      releases: [
        status: [
          include_executables_for: [:unix],
          strip_beams: true
          # No cookie is set here. `mix release` makes a random one at build time and puts it in the
          # release folder (releases/COOKIE). Never commit a built release.
        ]
      ]
    ]
  end

  def application do
    [extra_applications: [:logger], mod: {Status.Application, []}]
  end
end
