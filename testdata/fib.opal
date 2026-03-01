var n = 10;
func fib(n) {
    if n <= 1 then { return n; }
    return fib(n - 1) + fib(n - 2);
}
var result = fib(n);
