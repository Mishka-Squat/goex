package deque

import (
	"iter"
	"slices"

	"github.com/Mishka-Squat/goex/cache"
	"github.com/Mishka-Squat/goex/unsafeex"
)

type Of[T any] struct {
	page_size         int
	items_per_page    int
	pages             [][]T
	first_page_offset int // offset f first page data when making PopFront it increases
}

func Make[T any](page_size int) Of[T] {
	r := Of[T]{
		page_size: page_size,
	}
	tsizeof := int(unsafeex.Sizeof[T]())

	if tsizeof != 0 {
		r.items_per_page = page_size / tsizeof
		for r.items_per_page == 0 {
			page_size *= 2
			r.items_per_page = page_size / tsizeof
		}
	} else {
		// TODO(iga): need some special optimization for empty types
		r.items_per_page = r.page_size
	}

	return r
}

func MakePerfect[T any]() Of[T] {
	r := Of[T]{
		page_size: cache.PerfectPageSize[T](),
	}
	r.items_per_page = r.page_size / int(unsafeex.Sizeof[T]())

	return r
}

func (d Of[T]) firstPage() []T {
	if len(d.pages) == 0 {
		return nil
	}

	return d.pages[0]
}

func (d Of[T]) lastPage() []T {
	if len(d.pages) == 0 {
		return nil
	}

	return d.pages[len(d.pages)-1]
}

func (d Of[T]) makePage() []T {
	return make([]T, 0, d.items_per_page)
}

func (d Of[T]) Each() iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, page := range d.pages {
			for _, i := range page {
				if !yield(i) {
					return
				}
			}
		}
	}
}

func (d Of[T]) Empty() bool {
	switch len(d.pages) {
	case 0:
		return true
	case 1:
		return len(d.pages[0]) == 0
	default:
		return false
	}
}

func (d Of[T]) Len() int {
	l := max(0, len(d.pages)-1) * d.items_per_page
	if l > 1 {
		fpl := len(d.firstPage())
		l -= d.items_per_page - fpl // first page may not be full
	}
	l += len(d.lastPage())
	return l
}

func (d *Of[T]) Clear(capacity ...int) {
	page_index := 0
	item_index := 0
	if len(capacity) > 0 {
		cap := capacity[0]
		maxcap := len(d.pages) * d.items_per_page
		if cap < 0 {
			cap = 0
		} else if cap > maxcap {
			cap = maxcap
		}
		page_index = cap / d.items_per_page
		item_index = cap % d.items_per_page
		if item_index != 0 {
			page_index++
		}
	}

	for i := range d.pages {
		if i < page_index {
			p := d.pages[i]
			clear(p)
			d.pages[i] = p[0:0]
		} else {
			d.pages[i] = nil
		}
	}
	d.pages = d.pages[0:max(0, page_index-1)]
}

func (d Of[T]) Item(index int) *T {
	if d.items_per_page == 0 {
		return nil
	}

	fpo := d.first_page_offset
	page_index := index / (d.items_per_page + fpo)
	item_index := index % (d.items_per_page + fpo)
	if index < d.items_per_page-fpo {
		item_index = index
	}

	if len(d.pages) <= page_index {
		return nil
	}

	page := d.pages[page_index]
	return &page[item_index]
}

func (d Of[T]) Last() *T {
	lp := d.lastPage()
	if len(lp) == 0 {
		return nil
	}

	return &lp[len(lp)-1]
}

func (d Of[T]) Reserve(capacity int) Of[T] {
	page_index := capacity / d.items_per_page
	item_index := capacity % d.items_per_page
	if item_index != 0 {
		page_index++
	}

	if len(d.pages) < page_index {
		d.pages = slices.Grow(d.pages, page_index)
		for i := len(d.pages); i < page_index-1; i++ {
			d.pages = append(d.pages, d.makePage()[0:d.items_per_page])
		}
		if item_index == 0 {
			d.pages = append(d.pages, d.makePage()[0:d.items_per_page])
		} else {
			d.pages = append(d.pages, d.makePage()[0:min(d.items_per_page, item_index)])
		}
	}

	last_index := capacity - 1
	page_index = last_index / d.items_per_page
	item_index = last_index % d.items_per_page

	page := d.pages[page_index]
	d.pages[page_index] = page[0:max(len(page), item_index+1)]
	return d
}

/*
func (d Of[T]) ReserveBlock(size int) Of[T] {
	page_index := capacity / d.items_per_page
	item_index := capacity % d.items_per_page
	if item_index != 0 {
		page_index++
	}

	if len(d.pages) < page_index {
		d.pages = slices.Grow(d.pages, page_index)
		for i := len(d.pages); i < page_index-1; i++ {
			d.pages = append(d.pages, d.makePage()[0:d.items_per_page])
		}
		if item_index == 0 {
			d.pages = append(d.pages, d.makePage()[0:d.items_per_page])
		} else {
			d.pages = append(d.pages, d.makePage()[0:min(d.items_per_page, item_index)])
		}
	}

	last_index := capacity - 1
	page_index = last_index / d.items_per_page
	item_index = last_index % d.items_per_page

	page := d.pages[page_index]
	d.pages[page_index] = page[0:max(len(page), item_index+1)]
	return d
}
*/

func (d Of[T]) Append(v T) Of[T] {
	lastPage := d.lastPage()
	if cap(lastPage) == 0 || len(lastPage) == cap(lastPage) {
		d.pages = append(d.pages, make([]T, 0, d.items_per_page))
	}
	d.pages[len(d.pages)-1] = append(d.pages[len(d.pages)-1], v)
	return d
}

func (d Of[T]) RemoveSwapback(index int) (Of[T], T) {
	var t T
	dl := d.Len()

	if index >= dl {
		return d, t
	} else if index == dl-1 {
		t = d.Pop()
	} else {
		t = *d.Item(index)
		*d.Item(index) = d.Pop()
	}

	return d, t
}

func (d *Of[T]) Top() T {
	return *d.Last()
}

func (d *Of[T]) Push(v T) {
	*d = d.Append(v)
}

func (d *Of[T]) Pop() (v T) {
	lastPage := d.lastPage()
	if len(lastPage) > 0 {
		v = lastPage[len(lastPage)-1]
		lastPage = lastPage[:len(lastPage)-1]
		if len(lastPage) > 0 {
			d.pages[len(d.pages)-1] = lastPage
		} else {
			d.pages = d.pages[:len(d.pages)-1]
		}
	}
	return
}

func (d *Of[T]) PopFront() (v T) {
	firstPage := d.firstPage()
	if len(firstPage) > 0 {
		v = firstPage[d.first_page_offset]

		var et T
		firstPage[d.first_page_offset] = et

		d.first_page_offset++
		if d.first_page_offset == d.items_per_page {
			d.pages = slices.Delete(d.pages, 0, 1)
			d.first_page_offset = 0
		} else if d.first_page_offset == len(firstPage) {
			d.first_page_offset = 0
			firstPage = firstPage[0:0]
			d.pages[0] = firstPage
		}
	}
	return
}
