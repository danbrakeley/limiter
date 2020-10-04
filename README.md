# limiter

Usage: `limiter <max-concurrency>`

Where `<max-concurrency>` is the number of tasks to execute in parallel.

Tasks are pulled from stdin, and lines starting with `#` are skipped, so if you have a file `serial.sh` with:

```sh
#!/bin/bash

sleep 1
sleep 2
sleep 1
## don't forget to sleep here
sleep 6
sleep 1
sleep 1
sleep 2
#sleep 1
sleep 3
sleep 3
```

And you run:

```text
limiter 3 < serial.sh
```

you'll see something like this:

![limiter console demo](demo.gif)
