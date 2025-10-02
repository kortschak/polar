// Copyright ©2025 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ring implements a simple ring buffer.
package ring

type Buffer[T any] struct {
	data       []T
	head, tail int
}

func NewBuffer[T any](n int) *Buffer[T] {
	return &Buffer[T]{data: make([]T, n)}
}

func (r *Buffer[T]) Len() int {
	if r.head <= r.tail {
		return r.tail - r.head
	}
	return len(r.data) - r.head + r.tail
}

func (r *Buffer[T]) Size() int {
	return len(r.data)
}

func (r *Buffer[T]) Write(src []T) {
	if len(src) >= len(r.data) {
		r.head = 0
		r.tail = len(r.data)
		copy(r.data, src[len(src)-len(r.data):])
		return
	}
	n := copy(r.data[r.tail:], src)
	if len(src) <= n {
		r.tail += n
		return
	}
	r.tail = copy(r.data, src[n:])
	if r.tail > r.head {
		r.head = r.tail
	}
}

func (r *Buffer[T]) Read(dst []T) int {
	n := r.CopyTo(dst)
	r.Advance(n)
	return n
}

func (r *Buffer[T]) CopyTo(dst []T) int {
	if r.head < r.tail {
		return copy(dst, r.data[r.head:r.tail])
	}
	n := copy(dst, r.data[r.head:])
	n += copy(dst[n:], r.data[:r.tail])
	return n
}

func (r *Buffer[T]) Advance(n int) {
	if r.head < r.tail {
		r.head += min(n, r.tail-r.head)
		return
	}
	front := len(r.data) - r.head
	if n <= front {
		r.head += n
		return
	}
	r.head = n - front
}
