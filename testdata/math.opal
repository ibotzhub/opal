func factorial(n) {
    if n <= 1 then { return 1; }
    return n * factorial(n - 1);
}
var result = factorial(10);
