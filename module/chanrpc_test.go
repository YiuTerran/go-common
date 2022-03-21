package module_test

import (
	"fmt"
	"github.com/YiuTerran/go-common/module"
	"sync"
)

func Example() {
	s := module.NewServer()

	var wg sync.WaitGroup
	wg.Add(1)

	// goroutine 1
	go func() {
		s.Register("f0", func(args []any) {

		})

		s.Register("f1", func(args []any) any {
			return 1
		})

		s.Register("fn", func(args []any) []any {
			return []any{1, 2, 3}
		})

		s.Register("add", func(args []any) any {
			n1 := args[0].(int)
			n2 := args[1].(int)
			return n1 + n2
		})

		wg.Done()

		for {
			s.Exec(<-s.chanCall.Out)
		}
	}()

	wg.Wait()
	wg.Add(1)

	// goroutine 2
	go func() {
		c := s.Open(false)

		// sync
		err := c.Call0("f0")
		if err != nil {
			fmt.Println(err)
		}

		r1, err := c.Call1("f1")
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(r1)
		}

		rn, err := c.CallN("fn")
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(rn[0], rn[1], rn[2])
		}

		ra, err := c.Call1("add", 1, 2)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(ra)
		}

		// async
		c.AsyncCall("f0", func(err error) {
			if err != nil {
				fmt.Println(err)
			}
		})

		c.AsyncCall("f1", func(ret any, err error) {
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(ret)
			}
		})

		c.AsyncCall("fn", func(ret []any, err error) {
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(ret[0], ret[1], ret[2])
			}
		})

		c.AsyncCall("add", 1, 2, func(ret any, err error) {
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(ret)
			}
		})

		c.Cb(<-c.ChanAsyncRet.Out)
		c.Cb(<-c.ChanAsyncRet.Out)
		c.Cb(<-c.ChanAsyncRet.Out)
		c.Cb(<-c.ChanAsyncRet.Out)

		// g
		s.Go("f0")

		wg.Done()
	}()

	wg.Wait()

	// Output:
	// 1
	// 1 2 3
	// 3
	// 1
	// 1 2 3
	// 3
}
