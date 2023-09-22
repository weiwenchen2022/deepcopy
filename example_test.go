package deepcopy_test

import (
	"fmt"

	"github.com/weiwenchen2022/deepcopy"
)

func ExampleDeepCopy() {
	var dst = make([]int, 5)
	src := []int{1, 2, 3}
	deepcopy.DeepCopy(&dst, src)

	fmt.Println(len(dst), cap(dst))
	for _, v := range dst {
		fmt.Println(v)
	}

	// Output:
	// 3 3
	// 1
	// 2
	// 3
}

func ExampleDeepClone() {
	src := []int{1, 2, 3}
	dst := deepcopy.DeepClone(src).([]int)

	fmt.Println(len(dst), cap(dst))
	for _, v := range dst {
		fmt.Println(v)
	}

	// Output:
	// 3 3
	// 1
	// 2
	// 3
}
