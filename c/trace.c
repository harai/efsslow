#include <linux/dcache.h>
#include <linux/fs.h>
#include <linux/sched.h>
#include <uapi/linux/ptrace.h>

#define SLOW_THRESHOLD_MS /*SLOW_THRESHOLD_MS*/

#define SLOW_EVENT_NFS4_FILE_OPEN 0x1
#define SLOW_EVENT_NFS_FILE_OPEN 0x2

struct val_t {
  u64 ts;
  struct file *fp;
};

struct data_t {
  u64 type;
  u64 ts_us;
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

int return__nfs4_file_open(struct pt_regs *ctx) {
  return trace_exit(ctx, SLOW_EVENT_NFS4_FILE_OPEN);
}

int return__nfs_file_open(struct pt_regs *ctx) {
  return trace_exit(ctx, SLOW_EVENT_NFS_FILE_OPEN);
}
