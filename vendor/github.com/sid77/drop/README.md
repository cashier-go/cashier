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
