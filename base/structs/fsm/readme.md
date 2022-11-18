## FSM状态机

使用方法：

1. 定义State， Index表示当前状态，Outcome表示接收到Input时转换到的新状态；
2. Outcome的`Action`，表示新状态的回调函数，这个回调可以返回一个新的Input，用于链式触发；
3. 使用`Define`来将State串联起来转换成状态机；
4. 使用`Spin`来输入值，开始流转状态机；