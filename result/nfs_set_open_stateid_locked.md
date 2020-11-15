# `nfs_set_open_stateid_locked()`

`nfs_set_open_stateid_locked()` function makes opening EFS files slow.

## Stack

- N/S: No symbol (Unable to attach kprobe/kretprobe)
- Conditional: Called conditionally
- Callback: Called as a callback function

```text
nfs4_atomic_open
    nfs4_do_open (N/S)
        _nfs4_do_open (N/S)
            _nfs4_open_and_get_state (N/S)
                _nfs4_proc_open (N/S)
                _nfs4_opendata_to_nfs4_state
                    update_open_stateid
                        nfs_state_set_open_stateid (N/S)
                            nfs_set_open_stateid_locked (N/S)
                                                //
                                prepare_to_wait // Inside
                                finish_wait     // For loop
                                                //
                                nfs_state_log_update_open_stateid (Conditional)
                        nfs_mark_delegation_referenced (Conditional)
                        update_open_stateflags
```

## Log lines

- `delta`
  - Total time elapsed in `nfs4_file_open()`
- `points`
  - Entered/returned history of probed functions
    - `id`: A identification number attached to a probe
    - `delta`: Time elapsed since entered `nfs4_file_open()`
    - `total`: The total number this probe is called
- `counts`
  - The total number of calls for each probe, sorted by IDs
- `pid`
  - PID
- `task`
  - Process name
- `file`
  - (Shortened) file name

### Infinite loop

Execution got stuck in a loop defined at `nfs_set_open_stateid_locked()`.

```text
2020-11-15T13:52:13.681+0900    DEBUG   event   {"delta": "7.066236s", "points": [{"id": 0, "delta": "2µs", "total": 1}, {"id": 1, "delta": "3µs", "total": 1}, {"id": 2, "delta": "3µs", "total": 1}, {"id": 3, "delta": "4µs", "total": 1}, {"id": 4, "delta": "5µs", "total": 1}, {"id": 5, "delta": "5µs", "total": 1}, {"id": 6, "delta": "5µs", "total": 1}, {"id": 8, "delta": "14µs", "total": 255}, {"id": 9, "delta": "14µs", "total": 255}, {"id": 10, "delta": "1.932ms", "total": 255}, {"id": 11, "delta": "1.932ms", "total": 255}, {"id": 7, "delta": "1.935ms", "total": 1}, {"id": 8, "delta": "1.936ms", "total": 255}, {"id": 9, "delta": "1.936ms", "total": 255}, {"id": 10, "delta": "4.281ms", "total": 255}, {"id": 11, "delta": "4.282ms", "total": 255}, {"id": 8, "delta": "4.282ms", "total": 255}, {"id": 9, "delta": "4.283ms", "total": 255}, {"id": 10, "delta": "4.625ms", "total": 255}, {"id": 11, "delta": "4.626ms", "total": 255}, {"id": 8, "delta": "4.626ms", "total": 255}, {"id": 9, "delta": "4.626ms", "total": 255}, {"id": 10, "delta": "6.229ms", "total": 255}, {"id": 11, "delta": "6.23ms", "total": 255}, {"id": 8, "delta": "6.23ms", "total": 255}, {"id": 9, "delta": "6.231ms", "total": 255}, {"id": 10, "delta": "6.528ms", "total": 255}, {"id": 11, "delta": "6.529ms", "total": 255}, {"id": 8, "delta": "6.529ms", "total": 255}, {"id": 9, "delta": "6.53ms", "total": 255}, {"id": 10, "delta": "8.169ms", "total": 255}, {"id": 11, "delta": "8.169ms", "total": 255}, {"id": 8, "delta": "8.17ms", "total": 255}, {"id": 9, "delta": "8.171ms", "total": 255}, {"id": 10, "delta": "8.712ms", "total": 255}, {"id": 11, "delta": "8.712ms", "total": 255}, {"id": 8, "delta": "8.713ms", "total": 255}, {"id": 9, "delta": "8.713ms", "total": 255}, {"id": 10, "delta": "10.629ms", "total": 255}, {"id": 11, "delta": "10.629ms", "total": 255}, {"id": 8, "delta": "10.63ms", "total": 255}, {"id": 9, "delta": "10.631ms", "total": 255}, {"id": 10, "delta": "10.94ms", "total": 255}, {"id": 11, "delta": "10.941ms", "total": 255}, {"id": 8, "delta": "10.941ms", "total": 255}, {"id": 9, "delta": "10.941ms", "total": 255}, {"id": 10, "delta": "11.822ms", "total": 255}, {"id": 11, "delta": "11.822ms", "total": 255}, {"id": 8, "delta": "11.823ms", "total": 255}, {"id": 9, "delta": "11.823ms", "total": 255}, {"id": 10, "delta": "12.113ms", "total": 255}, {"id": 11, "delta": "12.113ms", "total": 255}, {"id": 8, "delta": "12.114ms", "total": 255}, {"id": 9, "delta": "12.114ms", "total": 255}, {"id": 10, "delta": "12.687ms", "total": 255}, {"id": 11, "delta": "12.688ms", "total": 255}, {"id": 8, "delta": "12.688ms", "total": 255}, {"id": 9, "delta": "12.689ms", "total": 255}, {"id": 10, "delta": "12.999ms", "total": 255}, {"id": 11, "delta": "13ms", "total": 255}, {"id": 8, "delta": "13ms", "total": 255}, {"id": 9, "delta": "13.001ms", "total": 255}, {"id": 10, "delta": "14.993ms", "total": 255}, {"id": 11, "delta": "14.994ms", "total": 255}, {"id": 8, "delta": "14.994ms", "total": 255}, {"id": 9, "delta": "14.995ms", "total": 255}, {"id": 10, "delta": "15.379ms", "total": 255}, {"id": 11, "delta": "15.379ms", "total": 255}, {"id": 8, "delta": "15.38ms", "total": 255}, {"id": 9, "delta": "15.38ms", "total": 255}, {"id": 10, "delta": "16.921ms", "total": 255}, {"id": 11, "delta": "16.922ms", "total": 255}, {"id": 8, "delta": "16.922ms", "total": 255}, {"id": 9, "delta": "16.923ms", "total": 255}, {"id": 10, "delta": "17.25ms", "total": 255}, {"id": 11, "delta": "17.25ms", "total": 255}, {"id": 8, "delta": "17.251ms", "total": 255}, {"id": 9, "delta": "17.251ms", "total": 255}, {"id": 10, "delta": "38.365ms", "total": 255}, {"id": 11, "delta": "38.366ms", "total": 255}, {"id": 8, "delta": "38.367ms", "total": 255}, {"id": 9, "delta": "38.367ms", "total": 255}, {"id": 10, "delta": "38.838ms", "total": 255}, {"id": 11, "delta": "38.838ms", "total": 255}, {"id": 8, "delta": "38.839ms", "total": 255}, {"id": 9, "delta": "38.839ms", "total": 255}, {"id": 10, "delta": "40.58ms", "total": 255}, {"id": 11, "delta": "40.58ms", "total": 255}, {"id": 8, "delta": "40.581ms", "total": 255}, {"id": 9, "delta": "40.581ms", "total": 255}, {"id": 10, "delta": "40.95ms", "total": 255}, {"id": 11, "delta": "40.95ms", "total": 255}, {"id": 8, "delta": "40.951ms", "total": 255}, {"id": 9, "delta": "40.951ms", "total": 255}, {"id": 10, "delta": "42.599ms", "total": 255}, {"id": 11, "delta": "42.6ms", "total": 255}, {"id": 8, "delta": "42.601ms", "total": 255}, {"id": 9, "delta": "42.601ms", "total": 255}, {"id": 10, "delta": "42.949ms", "total": 255}, {"id": 11, "delta": "42.949ms", "total": 255}, {"id": 8, "delta": "42.95ms", "total": 255}, {"id": 9, "delta": "42.95ms", "total": 255}, {"id": 10, "delta": "44.561ms", "total": 255}, {"id": 11, "delta": "44.562ms", "total": 255}, {"id": 8, "delta": "44.562ms", "total": 255}, {"id": 9, "delta": "44.563ms", "total": 255}, {"id": 10, "delta": "44.823ms", "total": 255}, {"id": 11, "delta": "44.824ms", "total": 255}, {"id": 8, "delta": "44.824ms", "total": 255}, {"id": 9, "delta": "44.825ms", "total": 255}, {"id": 10, "delta": "46.492ms", "total": 255}, {"id": 11, "delta": "46.493ms", "total": 255}, {"id": 8, "delta": "46.493ms", "total": 255}, {"id": 9, "delta": "46.494ms", "total": 255}, {"id": 10, "delta": "46.807ms", "total": 255}, {"id": 11, "delta": "46.808ms", "total": 255}, {"id": 8, "delta": "46.808ms", "total": 255}, {"id": 9, "delta": "46.808ms", "total": 255}, {"id": 10, "delta": "48.393ms", "total": 255}, {"id": 11, "delta": "48.393ms", "total": 255}, {"id": 8, "delta": "48.394ms", "total": 255}, {"id": 9, "delta": "48.394ms", "total": 255}, {"id": 10, "delta": "48.667ms", "total": 255}, {"id": 11, "delta": "48.667ms", "total": 255}, {"id": 8, "delta": "48.668ms", "total": 255}, {"id": 9, "delta": "48.668ms", "total": 255}, {"id": 10, "delta": "50.217ms", "total": 255}, {"id": 11, "delta": "50.217ms", "total": 255}], "counts": [1, 1, 1, 1, 1, 1, 1, 1, 255, 255, 255, 255, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], "pid": 3223, "task": "php-fpm", "file": "style.css"}
```

Another log:

```text
2020-11-15T14:20:37.874+0900    DEBUG   event   {"delta": "7.453595s", "points": [{"id": 0, "delta": "2µs", "total": 1}, {"id": 1, "delta": "3µs", "total": 1}, {"id": 2, "delta": "3µs", "total": 1}, {"id": 3, "delta": "4µs", "total": 1}, {"id": 4, "delta": "4µs", "total": 1}, {"id": 5, "delta": "5µs", "total": 1}, {"id": 6, "delta": "5µs", "total": 1}, {"id": 8, "delta": "10µs", "total": 255}, {"id": 9, "delta": "11µs", "total": 255}, {"id": 10, "delta": "1.432ms", "total": 255}, {"id": 11, "delta": "1.432ms", "total": 255}, {"id": 7, "delta": "1.435ms", "total": 1}, {"id": 8, "delta": "1.435ms", "total": 255}, {"id": 9, "delta": "1.436ms", "total": 255}, {"id": 10, "delta": "3.042ms", "total": 255}, {"id": 11, "delta": "3.042ms", "total": 255}, {"id": 8, "delta": "3.043ms", "total": 255}, {"id": 9, "delta": "3.043ms", "total": 255}, {"id": 10, "delta": "3.36ms", "total": 255}, {"id": 11, "delta": "3.361ms", "total": 255}, {"id": 8, "delta": "3.361ms", "total": 255}, {"id": 9, "delta": "3.362ms", "total": 255}, {"id": 10, "delta": "4.861ms", "total": 255}, {"id": 11, "delta": "4.861ms", "total": 255}, {"id": 8, "delta": "4.862ms", "total": 255}, {"id": 9, "delta": "4.862ms", "total": 255}, {"id": 10, "delta": "5.173ms", "total": 255}, {"id": 11, "delta": "5.174ms", "total": 255}, {"id": 8, "delta": "5.174ms", "total": 255}, {"id": 9, "delta": "5.175ms", "total": 255}, {"id": 10, "delta": "6.746ms", "total": 255}, {"id": 11, "delta": "6.747ms", "total": 255}, {"id": 8, "delta": "6.748ms", "total": 255}, {"id": 9, "delta": "6.748ms", "total": 255}, {"id": 10, "delta": "7.027ms", "total": 255}, {"id": 11, "delta": "7.027ms", "total": 255}, {"id": 8, "delta": "7.028ms", "total": 255}, {"id": 9, "delta": "7.028ms", "total": 255}, {"id": 10, "delta": "8.552ms", "total": 255}, {"id": 11, "delta": "8.553ms", "total": 255}, {"id": 8, "delta": "8.554ms", "total": 255}, {"id": 9, "delta": "8.554ms", "total": 255}, {"id": 10, "delta": "8.809ms", "total": 255}, {"id": 11, "delta": "8.809ms", "total": 255}, {"id": 8, "delta": "8.81ms", "total": 255}, {"id": 9, "delta": "8.81ms", "total": 255}, {"id": 10, "delta": "10.354ms", "total": 255}, {"id": 11, "delta": "10.355ms", "total": 255}, {"id": 8, "delta": "10.356ms", "total": 255}, {"id": 9, "delta": "10.356ms", "total": 255}, {"id": 10, "delta": "10.682ms", "total": 255}, {"id": 11, "delta": "10.682ms", "total": 255}, {"id": 8, "delta": "10.683ms", "total": 255}, {"id": 9, "delta": "10.683ms", "total": 255}, {"id": 10, "delta": "12.265ms", "total": 255}, {"id": 11, "delta": "12.265ms", "total": 255}, {"id": 8, "delta": "12.265ms", "total": 255}, {"id": 9, "delta": "12.266ms", "total": 255}, {"id": 10, "delta": "12.655ms", "total": 255}, {"id": 11, "delta": "12.655ms", "total": 255}, {"id": 8, "delta": "12.656ms", "total": 255}, {"id": 9, "delta": "12.656ms", "total": 255}, {"id": 10, "delta": "14.17ms", "total": 255}, {"id": 11, "delta": "14.17ms", "total": 255}, {"id": 8, "delta": "14.171ms", "total": 255}, {"id": 9, "delta": "14.171ms", "total": 255}, {"id": 10, "delta": "14.448ms", "total": 255}, {"id": 11, "delta": "14.448ms", "total": 255}, {"id": 8, "delta": "14.449ms", "total": 255}, {"id": 9, "delta": "14.449ms", "total": 255}, {"id": 10, "delta": "35.503ms", "total": 255}, {"id": 11, "delta": "35.503ms", "total": 255}, {"id": 8, "delta": "35.505ms", "total": 255}, {"id": 9, "delta": "35.505ms", "total": 255}, {"id": 10, "delta": "35.843ms", "total": 255}, {"id": 11, "delta": "35.844ms", "total": 255}, {"id": 8, "delta": "35.845ms", "total": 255}, {"id": 9, "delta": "35.845ms", "total": 255}, {"id": 10, "delta": "37.759ms", "total": 255}, {"id": 11, "delta": "37.76ms", "total": 255}, {"id": 8, "delta": "37.76ms", "total": 255}, {"id": 9, "delta": "37.761ms", "total": 255}, {"id": 10, "delta": "38.102ms", "total": 255}, {"id": 11, "delta": "38.102ms", "total": 255}, {"id": 8, "delta": "38.103ms", "total": 255}, {"id": 9, "delta": "38.103ms", "total": 255}, {"id": 10, "delta": "39.711ms", "total": 255}, {"id": 11, "delta": "39.712ms", "total": 255}, {"id": 8, "delta": "39.712ms", "total": 255}, {"id": 9, "delta": "39.713ms", "total": 255}, {"id": 10, "delta": "40.034ms", "total": 255}, {"id": 11, "delta": "40.034ms", "total": 255}, {"id": 8, "delta": "40.036ms", "total": 255}, {"id": 9, "delta": "40.036ms", "total": 255}, {"id": 10, "delta": "41.709ms", "total": 255}, {"id": 11, "delta": "41.709ms", "total": 255}, {"id": 8, "delta": "41.71ms", "total": 255}, {"id": 9, "delta": "41.711ms", "total": 255}, {"id": 10, "delta": "42.011ms", "total": 255}, {"id": 11, "delta": "42.012ms", "total": 255}, {"id": 8, "delta": "42.012ms", "total": 255}, {"id": 9, "delta": "42.013ms", "total": 255}, {"id": 10, "delta": "44.074ms", "total": 255}, {"id": 11, "delta": "44.074ms", "total": 255}, {"id": 8, "delta": "44.075ms", "total": 255}, {"id": 9, "delta": "44.075ms", "total": 255}, {"id": 10, "delta": "44.429ms", "total": 255}, {"id": 11, "delta": "44.43ms", "total": 255}, {"id": 8, "delta": "44.43ms", "total": 255}, {"id": 9, "delta": "44.431ms", "total": 255}, {"id": 10, "delta": "46.298ms", "total": 255}, {"id": 11, "delta": "46.298ms", "total": 255}, {"id": 8, "delta": "46.299ms", "total": 255}, {"id": 9, "delta": "46.3ms", "total": 255}, {"id": 10, "delta": "46.63ms", "total": 255}, {"id": 11, "delta": "46.631ms", "total": 255}, {"id": 8, "delta": "46.632ms", "total": 255}, {"id": 9, "delta": "46.633ms", "total": 255}, {"id": 10, "delta": "48.91ms", "total": 255}, {"id": 11, "delta": "48.91ms", "total": 255}, {"id": 8, "delta": "48.911ms", "total": 255}, {"id": 9, "delta": "48.912ms", "total": 255}, {"id": 10, "delta": "49.309ms", "total": 255}, {"id": 11, "delta": "49.309ms", "total": 255}, {"id": 8, "delta": "49.31ms", "total": 255}, {"id": 9, "delta": "49.31ms", "total": 255}, {"id": 10, "delta": "51.071ms", "total": 255}, {"id": 11, "delta": "51.071ms", "total": 255}], "counts": [1, 1, 1, 1, 1, 1, 1, 1, 255, 255, 255, 255, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], "pid": 6300, "task": "php-fpm", "file": "style.css"}
```

### Just takes time

Huge gap between `prepare_to_wait()` and `finish_wait()` function calls, probably under `nfs_set_open_stateid_locked()` function.

```text
2020-11-15T14:20:33.265+0900    DEBUG   event   {"delta": "5.229859s", "points": [{"id": 0, "delta": "2µs", "total": 1}, {"id": 1, "delta": "3µs", "total": 1}, {"id": 2, "delta": "3µs", "total": 1}, {"id": 3, "delta": "4µs", "total": 1}, {"id": 4, "delta": "4µs", "total": 1}, {"id": 5, "delta": "5µs", "total": 1}, {"id": 6, "delta": "5µs", "total": 1}, {"id": 8, "delta": "7µs", "total": 2}, {"id": 9, "delta": "8µs", "total": 2}, {"id": 10, "delta": "1.42ms", "total": 2}, {"id": 11, "delta": "1.421ms", "total": 2}, {"id": 7, "delta": "1.423ms", "total": 1}, {"id": 8, "delta": "1.424ms", "total": 2}, {"id": 9, "delta": "1.424ms", "total": 2}, {"id": 10, "delta": "5.229838s", "total": 2}, {"id": 11, "delta": "5.229841s", "total": 2}, {"id": 12, "delta": "5.229843s", "total": 1}, {"id": 13, "delta": "5.229844s", "total": 1}, {"id": 14, "delta": "5.229846s", "total": 1}, {"id": 15, "delta": "5.229847s", "total": 1}, {"id": 16, "delta": "5.229848s", "total": 1}, {"id": 17, "delta": "5.229856s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}], "counts": [1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], "pid": 6301, "task": "php-fpm", "file": "User_Aware_Trait.php"}
```

## Probes

- `enter__` and `return__` mean kprobe and kretprobe respectively.
- The rest of function name is the function name to be probed.
- The second argument of `check()` denotes its ID.

```c
int enter__nfs4_atomic_open(struct pt_regs *ctx) { return check(ctx, 0); }

int enter__nfs4_client_recover_expired_lease(struct pt_regs *ctx) {
  return check(ctx, 1);
}
int enter__nfs4_wait_clnt_recover(struct pt_regs *ctx) { return check(ctx, 2); }
int enter__nfs_wait_bit_killable(struct pt_regs *ctx) { return check(ctx, 18); }
int return__nfs_wait_bit_killable(struct pt_regs *ctx) { return check(ctx, 19); }
int enter__nfs_put_client(struct pt_regs *ctx) { return check(ctx, 3); }
int return__nfs_put_client(struct pt_regs *ctx) { return check(ctx, 4); }
int return__nfs4_wait_clnt_recover(struct pt_regs *ctx) { return check(ctx, 5); }
int return__nfs4_client_recover_expired_lease(struct pt_regs *ctx) {
  return check(ctx, 6);
}
int enter__update_open_stateid(struct pt_regs *ctx) { return check(ctx, 7); }
int enter__prepare_to_wait(struct pt_regs *ctx) { return check(ctx, 8); }
int return__prepare_to_wait(struct pt_regs *ctx) { return check(ctx, 9); }
int enter__finish_wait(struct pt_regs *ctx) { return check(ctx, 10); }
int return__finish_wait(struct pt_regs *ctx) { return check(ctx, 11); }
int enter__nfs_state_log_update_open_stateid(struct pt_regs *ctx) {
  return check(ctx, 12);
}
int return__nfs_state_log_update_open_stateid(struct pt_regs *ctx) {
  return check(ctx, 13);
}
int enter__update_open_stateflags(struct pt_regs *ctx) { return check(ctx, 14); }
int return__update_open_stateflags(struct pt_regs *ctx) { return check(ctx, 15); }
int return__update_open_stateid(struct pt_regs *ctx) { return check(ctx, 16); }
int return__nfs4_atomic_open(struct pt_regs *ctx) { return check(ctx, 17); }
```
