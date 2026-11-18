{
  pkgs,
  inputs,
  ...
}:
{
  packages = with pkgs; [
    # Nix LSP: picked up by editors for devenv.nix
    nixd
    # Git hooks (previously a devcontainer feature)
    lefthook
  ];

  languages.go = {
    enable = true;
    # Rolling nixpkgs removed go_1_25 (EOL) and its newer Go breaks the pinned
    # golangci-lint v2.10.1 typecheck; pin Go 1.25 from stable nixpkgs instead.
    package = inputs.nixpkgs-go.legacyPackages.${pkgs.system}.go_1_25;
  };

  # Pinned quality toolchain (golangci-lint, govulncheck, gosec,
  # go-arch-lint, gremlins, goreleaser) via `go install` into a repo-local
  # bin: keeps the exact versions CI and the release workflow pin
  # (specs/release-process.md), which nixpkgs would drift from.
  tasks."reglint:setup" = {
    exec = ''
      cd "$DEVENV_ROOT"
      mkdir -p .cache/go-tools
      GOBIN="$DEVENV_ROOT/.cache/go-tools" bash scripts/install-go-tools.sh
      touch .cache/go-tools/.stamp
    '';
    # Up to date: stamp exists and the tool script has not changed since
    status = ''
      test -f .cache/go-tools/.stamp && ! test scripts/install-go-tools.sh -nt .cache/go-tools/.stamp
    '';
    before = [ "devenv:enterShell" ];
  };

  enterShell = ''
    export PATH="$DEVENV_ROOT/.cache/go-tools:$DEVENV_ROOT/scripts:$DEVENV_ROOT/bin:$PATH"
    # Conventional GOPATH/GOCACHE outside the repo: keeps repo walkers
    # (go-arch-lint, scans) out of the module cache and matches CI cache paths.
    export GOPATH="$HOME/go"
    export GOCACHE="$HOME/.cache/go-build"
    lefthook install >/dev/null 2>&1 || true
  '';
}
