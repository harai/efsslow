#include <linux/dcache.h>
#include <linux/fs.h>
#include <linux/sched.h>
#include <uapi/linux/ptrace.h>

#define SLOW_THRESHOLD_MS /*SLOW_THRESHOLD_MS*/
#define SLOW_POINT_COUNT 16
#define CALL_ORDER_COUNT 32

struct val_t {
  u64 ts;
  u64 points_delta[SLOW_POINT_COUNT];
  u8 points_count[SLOW_POINT_COUNT];
  u8 call_order[CALL_ORDER_COUNT];
  u64 order_index;
  u64 delta;
  u64 pid;
  char task[TASK_COMM_LEN];
  char file[DNAME_INLINE_LEN];
};

BPF_HASH(entryinfo, u64, struct val_t);
BPF_PERF_OUTPUT(events);

int enter__nfs4_file_open(struct pt_regs *ctx, struct inode *inode, struct file *filp) {
  u64 id = bpf_get_current_pid_tgid();

  struct val_t val = {.call_order = 0, .ts = bpf_ktime_get_ns() / 1000};

  bpf_probe_read_kernel(val.file, DNAME_INLINE_LEN, filp->f_path.dentry->d_iname);

#pragma unroll
  for (int i = 0; i < SLOW_POINT_COUNT; i++) {
    val.points_delta[i] = 0x0;
    val.points_count[i] = 0;
  }

#pragma unroll
  for (int i = 0; i < CALL_ORDER_COUNT; i++) {
    val.call_order[i] = 0;
  }

  entryinfo.update(&id, &val);

  return 0;
}

int return__nfs4_file_open(struct pt_regs *ctx) {
  u64 id = bpf_get_current_pid_tgid();
  struct val_t *valp = entryinfo.lookup(&id);
  if (valp == 0) {
    return 0;
  }
  entryinfo.delete(&id);

  u64 delta = bpf_ktime_get_ns() / 1000 - valp->ts;
  if (delta < SLOW_THRESHOLD_MS * 1000) {
    return 0;
  }

  valp->delta = delta;
  valp->pid = id >> 32;

  bpf_get_current_comm(valp->task, sizeof(valp->task));

  events.perf_submit(ctx, valp, sizeof(struct val_t));
  return 0;
}

static int check(struct pt_regs *ctx, u8 point) {
  u64 id = bpf_get_current_pid_tgid();
  struct val_t *valp = entryinfo.lookup(&id);
  if (valp == 0) {
    return 0;
  }

  valp->points_delta[point] = bpf_ktime_get_ns() / 1000 - valp->ts;
  valp->points_count[point] += 1;

  valp->call_order[valp->order_index++ & (CALL_ORDER_COUNT - 1)] = point;

  entryinfo.update(&id, valp);
  return 0;
}

int enter__nfs4_atomic_open(struct pt_regs *ctx) { return check(ctx, 0); }

int enter__nfs4_client_recover_expired_lease(struct pt_regs *ctx) {
  return check(ctx, 1);
}
int enter__nfs4_wait_clnt_recover(struct pt_regs *ctx) { return check(ctx, 2); }
// DELAY
int enter__nfs_put_client(struct pt_regs *ctx) { return check(ctx, 3); }
int return__nfs_put_client(struct pt_regs *ctx) { return check(ctx, 4); }
int return__nfs4_wait_clnt_recover(struct pt_regs *ctx) { return check(ctx, 5); }
int return__nfs4_client_recover_expired_lease(struct pt_regs *ctx) {
  return check(ctx, 6);
}
int enter__update_open_stateid(struct pt_regs *ctx) { return check(ctx, 7); }
// DELAY
int enter__prepare_to_wait(struct pt_regs *ctx) { return check(ctx, 8); }
int return__prepare_to_wait(struct pt_regs *ctx) { return check(ctx, 9); }
int enter__nfs_state_log_update_open_stateid(struct pt_regs *ctx) {
  return check(ctx, 10);
}
int return__nfs_state_log_update_open_stateid(struct pt_regs *ctx) {
  return check(ctx, 11);
}
// DELAY
int enter__update_open_stateflags(struct pt_regs *ctx) { return check(ctx, 12); }
int return__update_open_stateflags(struct pt_regs *ctx) { return check(ctx, 13); }
int return__update_open_stateid(struct pt_regs *ctx) { return check(ctx, 14); }
int return__nfs4_atomic_open(struct pt_regs *ctx) { return check(ctx, 15); }
