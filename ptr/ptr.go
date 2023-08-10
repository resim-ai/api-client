// Package ptr contains a small utility function for inlining pointers.
//
// Suppose you have a struct like so:
//
//		type Foo struct {
//	 	Bar *string
//	 }
//
// To initialize one, you would have to write:
//
//	foo := Foo{
//		Bar: new(string)
//	}
//	*foo.Bar = "foobar"
//
// This is gross, especially for structs with a lot of members.  To alleviate this, we introduce the Ptr function:
//
//	 import . "github.com/resim-ai/rerun/ptr"
//		foo := Foo{
//			Bar: Ptr("foobar")
//		}
//
// Why can't I just use &?
//
// Golang forbids you from taking the address of a constant.  It must be assigned to a variable, then you can
// take the address of that variable.  Luckily, function parameters count.
// And since Go is garbage collected, scoping issues aren't a problem.
//
// We recommend importing ptr as
//
//	import . "github.com/resim-ai/api-client/ptr"
//
// so you don't have to type `ptr.Ptr`.
package ptr

// Ptr takes its argument and returns a pointer to it.
func Ptr[T any](t T) *T {
	return &t
}
