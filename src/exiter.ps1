exit 0
# The reason for needing this file is to make `bosh export-release` happy.
# BOSH doesn't know what job to export, so it tries to export a windows job
# against a linux stemcell. This is a file that will cause a bash script to
# immediately exit (such as when exporting a release) but be ignored when run
# in powershell.
