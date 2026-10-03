# Third-party / interoperability notices

## ConverterPIX

ETS2OMSI uses **ConverterPIX** by mwl4 as an external interoperability helper to decode SCS binary PMD/PMG assets into PIX intermediate files. ConverterPIX is a separate GNU LGPL project and is not embedded in this source tree by default.

Project/source: https://github.com/mwl4/ConverterPIX

ETS2OMSI can download the Windows x64 binary into its local `tools` directory on first 3D preview/conversion. The downloaded helper remains a separate executable.

## SCS archive helpers

ZIP-based `.scs` traffic mods are read natively. HashFS packages can use SCS Software's Game Archive Packer or a compatible SCS extraction helper. These are not game assets and do not require an ETS2 installation.

ETS2OMSI does not distribute ETS2, DLC, mod vehicle assets or OMSI stock resources.

## go-webview2

The desktop window uses [go-webview2](https://github.com/jchv/go-webview2) (MIT) and the Microsoft Edge WebView2 runtime that ships with Windows 10/11.

