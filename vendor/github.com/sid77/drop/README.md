# drop

An easy way for dropping privileges in Go.


```
import "github.com/sid77/drop"

// privileged code here
// ...

if err := drop.DropPrivileges("some user"); err != nil {
        log.Fatal(err)
}

// unprivileged code here
// ...
```

drop will take care of calling `setre{s}[g,u]id()` depending on the platform it's being run on.

Beware that if Go coroutines are created *before* dropping the program privileges, some of them may retain the original permissions. This is a [limitation of the Go runtime itself](https://github.com/golang/go/issues/1435).
