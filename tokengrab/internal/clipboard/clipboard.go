// Package clipboard wraps atotto/clipboard for the single use-case of pushing
// a captured token to the system clipboard.
package clipboard

import "github.com/atotto/clipboard"

func Set(text string) error {
	return clipboard.WriteAll(text)
}
