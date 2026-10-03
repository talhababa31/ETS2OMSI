# Third-party / interoperability notices

## ConverterPIX

ETS2OMSI V2 uses **ConverterPIX** by mwl4 as an external interoperability helper to decode SCS binary PMD/PMG assets into PIX intermediate files. ConverterPIX is a separate GNU LGPL project and is not embedded in this source tree by default.

Project/source: https://github.com/mwl4/ConverterPIX

V2 can download the Windows x64 binary into its local `tools` directory on first 3D preview/conversion. The downloaded helper remains a separate executable.

## SCS archive helpers

ZIP-based `.scs` traffic mods are read natively. HashFS packages can use SCS Software's Game Archive Packer or a compatible SCS extraction helper. These are not game assets and do not require an ETS2 installation.

ETS2OMSI V2 does not distribute ETS2, DLC, mod vehicle assets or OMSI stock resources.
