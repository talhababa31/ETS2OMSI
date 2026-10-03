# Changelog

## V2.2 MATERIAL + PHYSICS

- Fixed sparse PIM material indices so triangle material slots are never compressed or shifted.
- Prevented normal/mask/specular/reflection maps from being promoted to visible diffuse textures.
- Wheel models now inherit the same exact slot-safe material pipeline.
- Added automatic OMSI AI physics profiles from vehicle name + measured model dimensions.
- Added estimated mass/profile to conversion result UI.
- Suppressed harmless optional ETS2 helper-material messages from user-facing warnings.
- Kept SCS-only / cars-only / exterior-only scope.
