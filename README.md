# opal

a scripting language runtime. started by kayos, finished by ibot.

kayos+ibot 5evr.

## build

```
make          # generate asm stubs + build binary
make test     # run all tests with -race
make generate # regenerate datum.s from asm.go (avo)
```

requires go 1.22+

## usage

```
opal script.opal   # run a script
opal               # interactive REPL
```

## language

### types

```
int    # 64-bit integer (well, Go int)
str    # utf-8 string
bool   # true / false
func   # first-class function value
```

### variables

```
var x = 42;
var name = "opal";
var ok = true;
```

### functions

```
func add(a, b) {
    return a + b;
}

var result = add(3, 4);
```

### control flow

```
if x > 0 then {
    exec echo positive;
} else {
    exec echo negative;
}

while n > 0 {
    var n = n - 1;
}

for i = 10 {
    var total = total + i;
}
```

### operators

```
+   -   *   /   %   # arithmetic (/ and % are integer ops)
+               # also string concat when either operand is a str
==  !=          # equality (works on any type via string representation)
<   >   <=  >=  # integer comparison
&&  ||          # logical
```

### process execution

```
exec ls -la;
exec ls | exec grep .go | exec wc -l;
bg server;      # fire and forget
exit;
```

### output

```
print x;
print "hello, " + name + "!";
```

### unary operators

```
var n = -5;
var ok = !false;
```

### string literals

```
var greeting = "hello, world";
var msg = "say \"hi\"";        # escape sequences supported
```

## design

the lexer uses a hand-written trie (`Token` → `Branch`) for keyword recognition,
with a single-byte avo-generated asm lookup table for single-character tokens.
the parser is a recursive descent over a fragment stream. the evaluator is a
tree-walking interpreter with a lexically-scoped variable system.

```
source → Fragger → Fragment stream → Parser → AST → Eval → os/exec
```

## notes

- no string indexing or slicing yet
- no i/o builtins beyond `print` and `exec`
- float literals not supported; all numbers are int
- type system is minimal by design: coercion is `int` → `bool` → `str`
