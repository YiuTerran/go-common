package chanutil

import "sync"

/**
  *  @author tryao
  *  @date 2022/03/25 10:44
**/

// Merge 将多个channel的数据合并到同一个channel
// 当chs都关闭之后，返回的channel会自动关闭
func Merge[T any](chs ...<-chan T) <-chan T {
	wg := new(sync.WaitGroup)
	out := make(chan T)

	pipe := func(ch <-chan T) {
		defer wg.Done()
		for t := range ch {
			out <- t
		}
	}

	wg.Add(len(chs))
	for _, ch := range chs {
		go pipe(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
