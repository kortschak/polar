package ring

import (
	"reflect"
	"testing"
)

var bufferTests = []struct {
	name string
	ops  func() any
	want any
}{
	{
		name: "new_4_uint16",
		ops: func() any {
			return NewBuffer[uint16](4)
		},
		want: &Buffer[uint16]{data: make([]uint16, 4)},
	},
	{
		name: "new_4_uint16_write_2",
		ops: func() any {
			r := NewBuffer[uint16](4)
			r.Write([]uint16{1, 2})
			return r
		},
		want: &Buffer[uint16]{data: []uint16{1, 2, 0, 0}, head: 0, tail: 2},
	},
	{
		name: "new_4_uint16_write_2_1",
		ops: func() any {
			r := NewBuffer[uint16](4)
			r.Write([]uint16{1, 2})
			r.Write([]uint16{3})
			return r
		},
		want: &Buffer[uint16]{data: []uint16{1, 2, 3, 0}, head: 0, tail: 3},
	},
	{
		name: "new_4_uint16_write_2_adv1_1",
		ops: func() any {
			r := NewBuffer[uint16](4)
			r.Write([]uint16{1, 2})
			r.Advance(1)
			r.Write([]uint16{3})
			return r
		},
		want: &Buffer[uint16]{data: []uint16{1, 2, 3, 0}, head: 1, tail: 3},
	},
	{
		name: "new_4_uint16_write_2_3",
		ops: func() any {
			r := NewBuffer[uint16](4)
			r.Write([]uint16{1, 2})
			r.Write([]uint16{3, 4, 5})
			return r
		},
		want: &Buffer[uint16]{data: []uint16{5, 2, 3, 4}, head: 1, tail: 1},
	},
	{
		name: "new_4_uint16_write_3_2",
		ops: func() any {
			r := NewBuffer[uint16](4)
			r.Write([]uint16{1, 2, 3})
			r.Write([]uint16{4, 5})
			return r
		},
		want: &Buffer[uint16]{data: []uint16{5, 2, 3, 4}, head: 1, tail: 1},
	},
	{
		name: "new_4_uint16_write_5",
		ops: func() any {
			r := NewBuffer[uint16](4)
			r.Write([]uint16{1, 2, 3, 4, 5})
			return r
		},
		want: &Buffer[uint16]{data: []uint16{2, 3, 4, 5}, head: 0, tail: 4},
	},
	{
		name: "new_4_uint16_write_4_adv2_1_read",
		ops: func() any {
			r := NewBuffer[uint16](4)
			r.Write([]uint16{1, 2, 3, 4})
			r.Advance(2)
			r.Write([]uint16{5})
			var buf [4]uint16
			n := r.Read(buf[:])
			return []any{r, buf[:n]}
		},
		want: []any{
			&Buffer[uint16]{data: []uint16{0x5, 0x2, 0x3, 0x4}, head: 1, tail: 1},
			[]uint16{0x3, 0x4, 0x5},
		},
	},
	{
		name: "new_4_uint16_write_4_adv2_1_copy",
		ops: func() any {
			r := NewBuffer[uint16](4)
			r.Write([]uint16{1, 2, 3, 4})
			r.Advance(2)
			r.Write([]uint16{5})
			var buf [4]uint16
			n := r.CopyTo(buf[:])
			return []any{r, buf[:n]}
		},
		want: []any{
			&Buffer[uint16]{data: []uint16{0x5, 0x2, 0x3, 0x4}, head: 2, tail: 1},
			[]uint16{0x3, 0x4, 0x5},
		},
	},
	{
		name: "head_one_before_end",
		ops: func() any {
			var buf [10]uint16
			r := &Buffer[uint16]{
				data: []uint16{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
				head: 7, tail: 3,
			}
			n := r.CopyTo(buf[:])
			return buf[:n]
		},
		want: []uint16{0x8, 0x1, 0x2, 0x3},
	},
	{
		name: "head_at_end",
		ops: func() any {
			var buf [10]uint16
			r := &Buffer[uint16]{
				data: []uint16{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
				head: 8, tail: 4,
			}
			n := r.CopyTo(buf[:])
			return buf[:n]
		},
		want: []uint16{0x1, 0x2, 0x3, 0x4},
	},
}

func TestBuffer(t *testing.T) {
	for _, test := range bufferTests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ops()
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("expected result:\ngot: %#v\nwant:%#v", got, test.want)
			}
		})
	}
}
