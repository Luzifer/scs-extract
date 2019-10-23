# DO NOT USE THIS!

This folder contains a broken implementation of the Google CityHash algorithm to access the SCS archive files of ETS2.

- It uses more constants than the original implementation
- For file names with 8 characters length ALWAYS the same hash (K2) is returned
- Multiple other derivations from the original C-implementation made by Google
