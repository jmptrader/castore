/*
Package andrew-d/castore contains a content-addressable storage implementation
for Go.  It allows inserting arbitrary data (as an io.Reader), and retrieving
data by key.  It attempts to be both configurable and usable by default - in
the base case, all the user must specify is the base path in which to store
files.
*/
package castore
