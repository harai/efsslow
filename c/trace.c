#include <linux/dcache.h>
#include <linux/fs.h>
#include <linux/sched.h>
#include <uapi/linux/ptrace.h>

#define SLOW_THRESHOLD_MS /*SLOW_THRESHOLD_MS*/
#define SLOW_POINT_COUNT 16

#define SLOW_EVENT_NFS4_FILE_OPEN 0x1
#define SLOW_EVENT_NFS_FILE_OPEN 0x2

struct val_t {
  u64 ts;
  u64 points_delta[SLOW_POINT_COUNT];
  u8 points_count[SLOW_POINT_COUNT];
  struct file *fp;
};

struct data_t {
  u64 type;
  u64 ts_us;
  u64 points_delta_us[SLOW_POINT_COUNT];
  u8 points_count[SLOW_POINT_COUNT];
  u64 delta_us;
  u64 pid;
  char task[TASK_COMM_LEN];
  char file[DNAME_INLINE_LEN];
};

BPF_HASH(entryinfo, u64, struct val_t);
BPF_PERF_OUTPUT(events);

int enter__nfs_file_open(struct pt_regs *ctx, struct inode *inode, struct file *filp) {
  u64 id = bpf_get_current_pid_tgid();

  struct val_t val = {};
  val.ts = bpf_ktime_get_ns();
  val.fp = filp;

#pragma unroll
  for (int i = 0; i < SLOW_POINT_COUNT; i++) {
    val.points_delta[i] = 0x0;
    val.points_count[i] = 0;
  }

  entryinfo.update(&id, &val);

  return 0;
}

static int trace_exit(struct pt_regs *ctx, u64 type) {
  u64 id = bpf_get_current_pid_tgid();
  struct val_t *valp = entryinfo.lookup(&id);
  if (valp == 0) {
    return 0;
  }
  entryinfo.delete(&id);

  u32 pid = id >> 32;

  u64 ts = bpf_ktime_get_ns();
  u64 delta_us = (ts - valp->ts) / 1000;
  if (delta_us < SLOW_THRESHOLD_MS * 1000) {
    return 0;
  }

  struct data_t data = {.type = type, .delta_us = delta_us, .pid = pid};

#pragma unroll
  for (int i = 0; i < SLOW_POINT_COUNT; i++) {
    data.points_delta_us[i] = valp->points_delta[i];
    data.points_count[i] = valp->points_count[i];
  }

  data.ts_us = ts / 1000;
  bpf_get_current_comm(&data.task, sizeof(data.task));

  // workaround (rewriter should handle file to d_name in one step):
  struct dentry *de = NULL;
  struct qstr qs = {};
  bpf_probe_read_kernel(&de, sizeof(de), &valp->fp->f_path.dentry);

  bpf_probe_read_kernel(&qs, sizeof(qs), (void *)&de->d_name);
  if (qs.len == 0)
    return 0;

  bpf_probe_read_kernel(&data.file, sizeof(data.file), (void *)qs.name);

  events.perf_submit(ctx, &data, sizeof(data));
  return 0;
}

static int check(struct pt_regs *ctx, int point) {
  u64 id = bpf_get_current_pid_tgid();
  struct val_t *valp = entryinfo.lookup(&id);
  if (valp == 0) {
    return 0;
  }

  u64 ts = bpf_ktime_get_ns();
  valp->points_delta[point] = (ts - valp->ts) / 1000;
  valp->points_count[point] += 1;

  entryinfo.update(&id, valp);
  return 0;
}

int return__nfs4_file_open(struct pt_regs *ctx) {
  return trace_exit(ctx, SLOW_EVENT_NFS4_FILE_OPEN);
}

int return__nfs_file_open(struct pt_regs *ctx) {
  return trace_exit(ctx, SLOW_EVENT_NFS_FILE_OPEN);
}

int enter__nfs4_atomic_open(struct pt_regs *ctx) { return check(ctx, 0); }

int enter__nfs4_get_state_owner(struct pt_regs *ctx) { return check(ctx, 1); }
int return__nfs4_get_state_owner(struct pt_regs *ctx) { return check(ctx, 2); }

int enter__nfs4_client_recover_expired_lease(struct pt_regs *ctx) {
  return check(ctx, 3);
}

int enter__nfs4_wait_clnt_recover(struct pt_regs *ctx) { return check(ctx, 4); }
// ======
int enter__out_of_line_wait_on_bit(struct pt_regs *ctx) { return check(ctx, 5); }
int return__out_of_line_wait_on_bit(struct pt_regs *ctx) { return check(ctx, 6); }

int enter__nfs_put_client(struct pt_regs *ctx) { return check(ctx, 7); }
int return__nfs_put_client(struct pt_regs *ctx) { return check(ctx, 8); }

int return__nfs4_wait_clnt_recover(struct pt_regs *ctx) { return check(ctx, 9); }

int enter__nfs4_schedule_state_manager(struct pt_regs *ctx) { return check(ctx, 10); }
int return__nfs4_schedule_state_manager(struct pt_regs *ctx) { return check(ctx, 11); }

int return__nfs4_client_recover_expired_lease(struct pt_regs *ctx) {
  return check(ctx, 12);
}

int enter__nfs4_run_open_task(struct pt_regs *ctx) { return check(ctx, 13); }
int return__nfs4_run_open_task(struct pt_regs *ctx) { return check(ctx, 14); }

int return__nfs4_atomic_open(struct pt_regs *ctx) { return check(ctx, 15); }
