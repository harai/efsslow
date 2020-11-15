#include <linux/dcache.h>
#include <linux/fs.h>
#include <linux/sched.h>
#include <uapi/linux/ptrace.h>

#define SLOW_THRESHOLD_MS /*SLOW_THRESHOLD_MS*/
#define SLOW_POINT_COUNT 32
#define CALL_ORDER_COUNT 128

struct data_t {
  u64 ts;
  u8 point_ids[CALL_ORDER_COUNT];
  u32 point_deltas[CALL_ORDER_COUNT];
  u8 call_counts[SLOW_POINT_COUNT];
  char task[TASK_COMM_LEN];
  char file[DNAME_INLINE_LEN];
  u64 pid;
  u64 delta;
  u64 order_index;
};

BPF_PERCPU_ARRAY(store, struct data_t, 1);
BPF_HASH(entryinfo, u64, struct data_t);
BPF_PERF_OUTPUT(events);

int enter__nfs4_file_open(struct pt_regs *ctx, struct inode *inode, struct file *filp) {
  int zero = 0;
  struct data_t *data = store.lookup(&zero);
  if (data == NULL) {
    return 0;
  }

  data->order_index = 0;
  data->ts = bpf_ktime_get_ns() / 1000;
  bpf_probe_read_kernel(data->file, DNAME_INLINE_LEN, filp->f_path.dentry->d_iname);

#pragma unroll
  for (int i = 0; i < SLOW_POINT_COUNT; i++) {
    data->call_counts[i] = 0;
  }

#pragma unroll
  for (int i = 0; i < CALL_ORDER_COUNT; i++) {
    data->point_ids[i] = 0;
    data->point_deltas[i] = 0;
  }

  u64 id = bpf_get_current_pid_tgid();
  entryinfo.update(&id, data);

  return 0;
}

int return__nfs4_file_open(struct pt_regs *ctx) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }
  entryinfo.delete(&id);

  data->pid = id >> 32;

  data->delta = bpf_ktime_get_ns() / 1000 - data->ts;
  if (data->delta < SLOW_THRESHOLD_MS * 1000) {
    return 0;
  }

  bpf_get_current_comm(data->task, sizeof(data->task));

  events.perf_submit(ctx, data, sizeof(struct data_t));
  return 0;
}

static int check(struct pt_regs *ctx, u8 point_id) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  if (data->call_counts[point_id] < 255) {
    data->call_counts[point_id]++;
  }

  if (data->order_index < CALL_ORDER_COUNT) {
    data->point_ids[data->order_index & (CALL_ORDER_COUNT - 1)] = point_id;
    data->point_deltas[data->order_index & (CALL_ORDER_COUNT - 1)] =
        bpf_ktime_get_ns() / 1000 - data->ts;
    data->order_index++;
  }

  entryinfo.update(&id, data);
  return 0;
}

int enter__nfs4_atomic_open(struct pt_regs *ctx) { return check(ctx, 0); }

int enter__nfs4_client_recover_expired_lease(struct pt_regs *ctx) {
  return check(ctx, 1);
}
int enter__nfs4_wait_clnt_recover(struct pt_regs *ctx) { return check(ctx, 2); }
// int enter__prepare_to_wait(struct pt_regs *ctx)
// int return__prepare_to_wait(struct pt_regs *ctx)
int enter__nfs_wait_bit_killable(struct pt_regs *ctx) { return check(ctx, 18); }
// DELAY
int return__nfs_wait_bit_killable(struct pt_regs *ctx) { return check(ctx, 19); }
// int enter__finish_wait(struct pt_regs *ctx)
// int return__finish_wait(struct pt_regs *ctx)
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

// DELAY
int enter__nfs_state_log_update_open_stateid(struct pt_regs *ctx) {
  return check(ctx, 12);
}
int return__nfs_state_log_update_open_stateid(struct pt_regs *ctx) {
  return check(ctx, 13);
}
// DELAY
int enter__update_open_stateflags(struct pt_regs *ctx) { return check(ctx, 14); }
int return__update_open_stateflags(struct pt_regs *ctx) { return check(ctx, 15); }
int return__update_open_stateid(struct pt_regs *ctx) { return check(ctx, 16); }
int return__nfs4_atomic_open(struct pt_regs *ctx) { return check(ctx, 17); }
