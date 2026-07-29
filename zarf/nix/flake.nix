{
  description = "Malina Go CLI and BUI";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      gomod2nix,
    }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: f system);
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          versionSource = builtins.readFile ../../sdk/malina/version.go;
          versionLines = builtins.filter builtins.isString (builtins.split "\n" versionSource);
          versionLine = builtins.head (
            builtins.filter (line: builtins.match "const Version = \"[^\"]+\"" line != null) versionLines
          );
          versionMatch = builtins.match "const Version = \"([^\"]+)\"" versionLine;

          malinaBase = gomod2nix.legacyPackages.${system}.buildGoApplication {
            pname = "malina";
            version = builtins.head versionMatch;
            src = ../../.;
            subPackages = [ "cmd/malina" ];
            modules = ./gomod2nix.toml;
            go = pkgs.go_1_26;
          };
        in
        {
          default = self.packages.${system}.cpu;

          # This wrapper supplies generic FFI/C++ runtime libraries only. It
          # does not contain stable-diffusion.cpp, its backends, or models.
          cpu = pkgs.symlinkJoin {
            name = "malina-${builtins.head versionMatch}";
            paths = [ malinaBase ];
            nativeBuildInputs = [ pkgs.makeWrapper ];
            postBuild = ''
              wrapProgram $out/bin/malina \
                --prefix ${pkgs.lib.optionalString pkgs.stdenv.isLinux "LD_LIBRARY_PATH"}${pkgs.lib.optionalString pkgs.stdenv.isDarwin "DYLD_LIBRARY_PATH"} : "${
                  pkgs.lib.makeLibraryPath [
                    pkgs.libffi
                    pkgs.stdenv.cc.cc.lib
                  ]
                }"
            '';
          };
        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          runtimeLibraries = [
            pkgs.libffi
            pkgs.stdenv.cc.cc.lib
          ];

          mkMalinaShell =
            {
              extraPackages ? [ ],
              extraLibraries ? [ ],
            }:
            pkgs.mkShell {
              packages = [
                pkgs.go_1_26
                pkgs.gopls
                pkgs.gotools
                pkgs.go-tools
                pkgs.staticcheck
                gomod2nix.legacyPackages.${system}.gomod2nix
                pkgs.pkg-config
                pkgs.libffi
                pkgs.nodejs_22
              ] ++ extraPackages;

              ${if pkgs.stdenv.isLinux then "LD_LIBRARY_PATH" else "DYLD_LIBRARY_PATH"} =
                pkgs.lib.makeLibraryPath (runtimeLibraries ++ extraLibraries);

              # MALINA_LIB deliberately remains a host-selected path. Only an
              # absolute directory is added to the Linux loader search path.
              shellHook = pkgs.lib.optionalString pkgs.stdenv.isLinux ''
                if [ -n "''${MALINA_LIB:-}" ]; then
                  if [ -d "$MALINA_LIB" ] && [ "''${MALINA_LIB#/}" != "$MALINA_LIB" ]; then
                    export LD_LIBRARY_PATH="$MALINA_LIB''${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
                  else
                    echo "malina nix: MALINA_LIB is not an absolute directory; LD_LIBRARY_PATH unchanged" >&2
                  fi
                fi
              '';
            };
        in
        {
          default = self.devShells.${system}.cpu;
          cpu = mkMalinaShell { };
          vulkan = mkMalinaShell {
            extraPackages = [ pkgs.vulkan-headers ];
            extraLibraries = [ pkgs.vulkan-loader ];
          };
        }
      );
    };
}
