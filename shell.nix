{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  name = "";
  packages = [ ];
  buildInputs = [ ];
  nativeBuildInputs = with pkgs; [
    go_1_22
  ];
}
