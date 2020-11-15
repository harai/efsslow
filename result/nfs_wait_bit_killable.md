# `nfs_wait_bit_killable()`

`nfs_wait_bit_killable()` callback function makes opening EFS files slow.

## Stack

- N/S: No symbol (Unable to set kprobe)
- Conditional: Called conditionally
- Callback: Called as a callback function

```text
nfs4_atomic_open
    nfs4_do_open (N/S)
        _nfs4_do_open (N/S)
            nfs4_get_state_owner
            nfs4_client_recover_expired_lease
                nfs4_wait_clnt_recover
                    wait_on_bit_action (N/S)
                        out_of_line_wait_on_bit
                            __wait_on_bit
                                prepare_to_wait
                                nfs_wait_bit_killable (Callback) <== This function call could be slow
                                finish_wait
                    nfs_put_client
                nfs4_schedule_state_manager
            nfs4_opendata_alloc
            _nfs4_open_and_get_state (N/S)
                _nfs4_proc_open (N/S)
                    nfs4_run_open_task
                        rpc_run_task
                        rpc_wait_for_completion_task (N/S)
                            __rpc_wait_for_completion_task
                    nfs_fattr_map_and_free_names
                    update_changeattr (Conditional)
                    _nfs4_proc_open_confirm (Conditional)
                    nfs4_proc_getattr (Conditional)
                _nfs4_opendata_to_nfs4_state
                    nfs4_opendata_find_nfs4_state (N/S)
                        nfs4_opendata_get_inode (N/S)
                    nfs4_opendata_check_deleg (N/S)
                        nfs_inode_set_delegation (Conditional)
                        nfs_inode_reclaim_delegation (Conditional)
                    update_open_stateid
                        nfs_state_set_open_stateid (N/S)
                            nfs_set_open_stateid_locked (N/S)
                                prepare_to_wait
                                nfs_test_and_clear_all_open_stateid (N/S)
                                nfs_state_log_update_open_stateid
                        nfs_mark_delegation_referenced (Conditional)
                        update_open_stateflags
                    nfs_release_seqid
                pnfs_parse_lgopen
                nfs4_opendata_access (N/S)
                nfs_inode_attach_open_context (Conditional)
```

## Log line

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

```text
2020-11-15T11:58:37.937+0900    DEBUG   event   {"delta": "5.064303s", "points": [{"id": 0, "delta": "2µs", "total": 1}, {"id": 1, "delta": "3µs", "total": 1}, {"id": 2, "delta": "3µs", "total": 1}, {"id": 8, "delta": "4µs", "total": 5}, {"id": 9, "delta": "5µs", "total": 5}, {"id": 18, "delta": "6µs", "total": 1}, {"id": 19, "delta": "1.031683s", "total": 1}, {"id": 10, "delta": "1.031687s", "total": 5}, {"id": 11, "delta": "1.031687s", "total": 5}, {"id": 3, "delta": "1.031688s", "total": 1}, {"id": 4, "delta": "1.031689s", "total": 1}, {"id": 5, "delta": "1.03169s", "total": 1}, {"id": 6, "delta": "1.031691s", "total": 1}, {"id": 8, "delta": "1.031703s", "total": 5}, {"id": 9, "delta": "1.031704s", "total": 5}, {"id": 10, "delta": "1.034837s", "total": 5}, {"id": 11, "delta": "1.034838s", "total": 5}, {"id": 7, "delta": "1.034844s", "total": 1}, {"id": 8, "delta": "1.034845s", "total": 5}, {"id": 9, "delta": "1.034846s", "total": 5}, {"id": 10, "delta": "5.063818s", "total": 5}, {"id": 11, "delta": "5.063819s", "total": 5}, {"id": 8, "delta": "5.06382s", "total": 5}, {"id": 9, "delta": "5.063821s", "total": 5}, {"id": 10, "delta": "5.063821s", "total": 5}, {"id": 11, "delta": "5.063821s", "total": 5}, {"id": 12, "delta": "5.063822s", "total": 1}, {"id": 13, "delta": "5.063825s", "total": 1}, {"id": 14, "delta": "5.063826s", "total": 1}, {"id": 15, "delta": "5.063826s", "total": 1}, {"id": 8, "delta": "5.063833s", "total": 5}, {"id": 9, "delta": "5.063833s", "total": 5}, {"id": 10, "delta": "5.064288s", "total": 5}, {"id": 11, "delta": "5.064289s", "total": 5}, {"id": 17, "delta": "5.064301s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}, {"id": 0, "delta": "0s", "total": 1}], "counts": [1, 1, 1, 1, 1, 1, 1, 1, 5, 5, 5, 5, 1, 1, 1, 1, 0, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], "pid": 25098, "task": "php-fpm", "file": "style.css"}
```

## Probes

- `enter__` and `return__` mean kprobe and kretprobe respectively
- The rest of function name is the function to be probed
- The second argument of `check()` denotes its ID

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
