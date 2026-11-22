package agilisads

import "context"

// Pagination is the pagination block of a list response.
type Pagination struct {
	Limit      int     `json:"limit"`
	Count      int     `json:"count"`
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor"`
}

// Iter walks every item of a list, fetching the next page when the current
// one is used up, until has_more is false:
//
//	it := client.Offers.ListAutoPaging(ctx, nil)
//	for it.Next() {
//		offer := it.Current()
//		...
//	}
//	if err := it.Err(); err != nil { ... }
type Iter[T any] struct {
	ctx     context.Context
	fetch   func(ctx context.Context, cursor string) ([]T, Pagination, error)
	items   []T
	index   int
	cursor  string
	started bool
	done    bool
	current T
	err     error
	pages   int
}

func newIter[T any](ctx context.Context, cursor string, fetch func(ctx context.Context, cursor string) ([]T, Pagination, error)) *Iter[T] {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Iter[T]{ctx: ctx, fetch: fetch, cursor: cursor}
}

// Next moves to the next item and reports whether there is one. It returns
// false at the end of the list or on an error; check Err afterwards.
func (it *Iter[T]) Next() bool {
	for {
		if it.err != nil {
			return false
		}
		if it.index < len(it.items) {
			it.current = it.items[it.index]
			it.index++
			return true
		}
		if it.done {
			return false
		}
		if it.started && it.cursor == "" {
			it.done = true
			return false
		}
		it.started = true
		items, p, err := it.fetch(it.ctx, it.cursor)
		if err != nil {
			it.err = err
			return false
		}
		it.pages++
		it.items, it.index = items, 0
		if !p.HasMore || p.NextCursor == nil || *p.NextCursor == "" || *p.NextCursor == it.cursor {
			it.done = true
		} else {
			it.cursor = *p.NextCursor
		}
	}
}

// Current returns the item Next moved to.
func (it *Iter[T]) Current() T { return it.current }

// Err returns the error that stopped the iteration, if any.
func (it *Iter[T]) Err() error { return it.err }

// Pages returns how many pages have been fetched so far.
func (it *Iter[T]) Pages() int { return it.pages }
