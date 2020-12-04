#include <linux/dcache.h>
#include <linux/fs.h>
#include <linux/nfs4.h>
#include <linux/nfs_fs.h>
#include <linux/sched.h>
#include <uapi/linux/nfs4.h>
#include <uapi/linux/ptrace.h>

#define SLOW_THRESHOLD_MS /*SLOW_THRESHOLD_MS*/
#define SAMPLE_RATIO      /*SAMPLE_RATIO*/
#define SLOW_POINT_COUNT 48
#define CALL_ORDER_COUNT 128

struct nfs4_state {
  struct list_head open_states;  /* List of states for the same state_owner */
  struct list_head inode_states; /* List of states for the same inode */
  struct list_head lock_states;  /* List of subservient lock stateids */

  struct nfs4_state_owner *owner; /* Pointer to the open owner */
  struct inode *inode;            /* Pointer to the inode */

  unsigned long flags;   /* Do we hold any locks? */
  spinlock_t state_lock; /* Protects the lock_states list */

  seqlock_t seqlock;         /* Protects the stateid/open_stateid */
  nfs4_stateid stateid;      /* Current stateid: may be delegation */
  nfs4_stateid open_stateid; /* OPEN stateid */

  /* The following 3 fields are protected by owner->so_lock */
  unsigned int n_rdonly; /* Number of read-only references */
  unsigned int n_wronly; /* Number of write-only references */
  unsigned int n_rdwr;   /* Number of read/write references */
  fmode_t state;         /* State on the server (R,W, or RW) */
  refcount_t count;

  wait_queue_head_t waitq;
  struct rcu_head rcu_head;
};

struct nfs4_opendata {
  struct kref kref;
  struct nfs_openargs o_arg;
  struct nfs_openres o_res;
  struct nfs_open_confirmargs c_arg;
  struct nfs_open_confirmres c_res;
  struct nfs4_string owner_name;
  struct nfs4_string group_name;
  struct nfs4_label *a_label;
  struct nfs_fattr f_attr;
  struct nfs4_label *f_label;
  struct dentry *dir;
  struct dentry *dentry;
  struct nfs4_state_owner *owner;
  struct nfs4_state *state;
  struct iattr attrs;
  struct nfs4_layoutget *lgp;
  unsigned long timestamp;
  bool rpc_done;
  bool file_created;
  bool is_recover;
  bool cancelled;
  int rpc_status;
};

enum {
  EFSS_OUTPUT_SAMPLE = 0,
  EFSS_OUTPUT_SLOW,
  EFSS_OUTPUT_SEQID,
  EFSS_OUTPUT_RECLAIM_NOGRACE,
  EFSS_OUTPUT_FILE,
};

struct data_fh_t {
  u8 size;
  char data[NFS_MAXFHSIZE];
} __attribute__((__packed__));

struct data_stateid_t {
  char seqid[4];
  char other[NFS4_STATEID_OTHER_SIZE];
  u32 type;
} __attribute__((__packed__));

struct data_state_t {
  struct data_stateid_t open_stateid;
  struct data_stateid_t stateid;
  u64 flags;
  u32 n_rdonly;
  u32 n_wronly;
  u32 n_rdwr;
  u32 state;
} __attribute__((__packed__));

struct data_client_t {
  u64 cl_state;
} __attribute__((__packed__));

struct data_run_open_task_t {
  struct data_fh_t enter_o_arg_fh;
  u32 enter_o_arg_share_access;
  u8 enter_o_arg_claim;
  struct data_stateid_t return_o_res_stateid;
} __attribute__((__packed__));

struct data_opendata_to_nfs4_state_t {
  struct data_stateid_t o_res_stateid;
} __attribute__((__packed__));

struct data_update_open_stateid_t {
  struct data_stateid_t open_stateid;
  struct data_state_t state;
} __attribute__((__packed__));

struct data_state_mark_reclaim_nograce_t {
  struct data_state_t enter_state;
  struct data_state_t return_state;
  u32 executed;
  u32 result;
} __attribute__((__packed__));

struct data_wait_clnt_recover_t {
  struct data_client_t client;
} __attribute__((__packed__));

struct data_t {
  u64 ts;
  u8 point_ids[CALL_ORDER_COUNT];
  u32 point_deltas[CALL_ORDER_COUNT];
  u8 call_counts[SLOW_POINT_COUNT];
  char task[TASK_COMM_LEN];
  char file[DNAME_INLINE_LEN];
  u64 pid;
  u64 delta;
  struct data_run_open_task_t run_open_task;
  struct data_opendata_to_nfs4_state_t opendata_to_nfs4_state;
  struct data_update_open_stateid_t update_open_stateid;
  struct data_state_mark_reclaim_nograce_t state_mark_reclaim_nograce;
  struct data_wait_clnt_recover_t wait_clnt_recover;
  u32 order_index;
  u32 reason;
} __attribute__((__packed__));

BPF_PERCPU_ARRAY(store, struct data_t, 1);
BPF_HASH(entryinfo, u64, struct data_t);
BPF_PERF_OUTPUT(events);

static void copy_fh(struct data_fh_t *dst, const struct nfs_fh *src) {
  bpf_probe_read_kernel(&dst->size, 1, &src->size);
  bpf_probe_read_kernel(dst->data, NFS_MAXFHSIZE, src->data);
}

static void copy_stateid(struct data_stateid_t *dst, const nfs4_stateid *src) {
  bpf_probe_read_kernel(dst->seqid, 4, (char *)(&src->seqid));
  bpf_probe_read_kernel(dst->other, NFS4_STATEID_OTHER_SIZE, src->other);
  bpf_probe_read_kernel(&dst->type, sizeof(u32), &src->type);
}

static void init_stateid(struct data_stateid_t *stateid) {
#pragma unroll
  for (int i = 0; i < 4; i++) {
    stateid->seqid[i] = 0;
  }
#pragma unroll
  for (int i = 0; i < NFS4_STATEID_OTHER_SIZE; i++) {
    stateid->other[i] = 0;
  }
  stateid->type = 0;
}

static void copy_state(struct data_state_t *dst, const struct nfs4_state *src) {
  copy_stateid(&dst->open_stateid, &src->open_stateid);
  copy_stateid(&dst->stateid, &src->stateid);
  bpf_probe_read_kernel(&dst->flags, sizeof(u64), &src->flags);
  bpf_probe_read_kernel(&dst->n_rdwr, sizeof(u32), &src->n_rdwr);
  bpf_probe_read_kernel(&dst->n_wronly, sizeof(u32), &src->n_wronly);
  bpf_probe_read_kernel(&dst->n_rdonly, sizeof(u32), &src->n_rdonly);
  bpf_probe_read_kernel(&dst->state, sizeof(u32), &src->state);
}

static void init_state(struct data_state_t *state) {
  init_stateid(&state->open_stateid);
  init_stateid(&state->stateid);
  state->flags = 0;
  state->n_rdwr = 0;
  state->n_wronly = 0;
  state->n_rdonly = 0;
  state->state = 0;
}

static void copy_client(struct data_client_t *dst, const struct nfs_client *src) {
  bpf_probe_read_kernel(&dst->cl_state, sizeof(u64), &src->cl_state);
}

static int followed_file(const char d_iname[]) {
  char name[] = "/*FILE_NAME*/";

  if (name[0] == '\0') {
    return 0;
  }

#pragma unroll
  for (int i = 0; i < DNAME_INLINE_LEN; i++) {
    if (name[i] != d_iname[i]) {
      return 0;
    }
    if (name[i] == '\0') {
      return 1;
    }
  }
  return 0;
}

int enter__nfs4_file_open(struct pt_regs *ctx, struct inode *inode, struct file *filp) {
  int zero = 0;
  struct data_t *data = store.lookup(&zero);
  if (data == NULL) {
    return 0;
  }

  data->order_index = 0;
  data->reason = 0x0;
  data->ts = bpf_ktime_get_ns() / 1000;
  bpf_probe_read_kernel(data->file, DNAME_INLINE_LEN, filp->f_path.dentry->d_iname);
  if (followed_file(data->file)) {
    data->reason |= 1 << EFSS_OUTPUT_FILE;
  }

#pragma unroll
  for (int i = 0; i < SLOW_POINT_COUNT; i++) {
    data->call_counts[i] = 0;
  }

#pragma unroll
  for (int i = 0; i < CALL_ORDER_COUNT; i++) {
    data->point_ids[i] = 0;
    data->point_deltas[i] = 0;
  }

  // Initialize conditional data
  data->state_mark_reclaim_nograce.executed = 0;
  data->state_mark_reclaim_nograce.result = 0;
  init_state(&data->state_mark_reclaim_nograce.enter_state);
  init_state(&data->state_mark_reclaim_nograce.return_state);

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

  if (SLOW_THRESHOLD_MS * 1000 <= data->delta) {
    data->reason |= 1 << EFSS_OUTPUT_SLOW;
  }

  if (bpf_get_prandom_u32() % SAMPLE_RATIO == 0) {
    data->reason |= 1 << EFSS_OUTPUT_SAMPLE;
  }

  if (data->reason == 0x0) {
    return 0;
  }

  bpf_get_current_comm(data->task, sizeof(data->task));

  events.perf_submit(ctx, data, sizeof(struct data_t));
  return 0;
}

static void add_data(struct data_t *data, u8 point_id) {
  if (data->call_counts[point_id] < 255) {
    data->call_counts[point_id]++;
  }

  if (data->order_index < CALL_ORDER_COUNT) {
    data->point_ids[data->order_index & (CALL_ORDER_COUNT - 1)] = point_id;
    data->point_deltas[data->order_index & (CALL_ORDER_COUNT - 1)] =
        bpf_ktime_get_ns() / 1000 - data->ts;
    data->order_index++;
  }
}

static int check(struct pt_regs *ctx, u8 point_id) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  add_data(data, point_id);

  entryinfo.update(&id, data);
  return 0;
}

BPF_HASH(nfs4_state_mark_reclaim_nograce_state, u64, struct nfs4_state *);

static int check_return_nfs4_state_mark_reclaim_nograce(struct pt_regs *ctx,
                                                        u8 point_id) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  struct nfs4_state **stpp = nfs4_state_mark_reclaim_nograce_state.lookup(&id);
  if (stpp == NULL) {
    return 0;
  }
  struct nfs4_state *state = *stpp;

  add_data(data, point_id);
  int ret = PT_REGS_RC(ctx);
  if (ret == 1) {
    data->reason |= 1 << EFSS_OUTPUT_RECLAIM_NOGRACE;
  }

  copy_state(&data->state_mark_reclaim_nograce.return_state, state);
  data->state_mark_reclaim_nograce.result = ret;

  nfs4_state_mark_reclaim_nograce_state.delete(&id);
  entryinfo.update(&id, data);
  return 0;
}

static int check_nfs4_opendata_alloc(struct pt_regs *ctx, u8 point_id) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  add_data(data, point_id);
  struct nfs4_opendata *opendata = (struct nfs4_opendata *)PT_REGS_RC(ctx);
  if (opendata != NULL) {
  }

  entryinfo.update(&id, data);
  return 0;
}

BPF_HASH(nfs4_run_open_task_opendata, u64, struct nfs4_opendata *);

static int check_enter_nfs4_run_open_task(struct pt_regs *ctx, u8 point_id,
                                          struct nfs4_opendata *opendata) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  add_data(data, point_id);
  copy_fh(&data->run_open_task.enter_o_arg_fh, opendata->o_arg.fh);
  bpf_probe_read_kernel(&data->run_open_task.enter_o_arg_share_access, sizeof(u32),
                        &opendata->o_arg.share_access);
  bpf_probe_read_kernel(&data->run_open_task.enter_o_arg_claim, sizeof(u8),
                        &opendata->o_arg.claim);

  nfs4_run_open_task_opendata.update(&id, &opendata);
  entryinfo.update(&id, data);
  return 0;
}

static int check__nfs4_opendata_to_nfs4_state(struct pt_regs *ctx, u8 point_id,
                                              struct nfs4_opendata *opendata) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  add_data(data, point_id);
  copy_stateid(&data->opendata_to_nfs4_state.o_res_stateid, &opendata->o_res.stateid);

  entryinfo.update(&id, data);
  return 0;
}

static int check__nfs4_wait_clnt_recover(struct pt_regs *ctx, u8 point_id,
                                         struct nfs_client *clp) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  add_data(data, point_id);
  copy_client(&data->wait_clnt_recover.client, clp);

  entryinfo.update(&id, data);
  return 0;
}

static int check_enter_nfs4_state_mark_reclaim_nograce(struct pt_regs *ctx, u8 point_id,
                                                       struct nfs4_state *state) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  add_data(data, point_id);
  copy_state(&data->state_mark_reclaim_nograce.enter_state, state);
  data->state_mark_reclaim_nograce.executed = 1;

  nfs4_state_mark_reclaim_nograce_state.update(&id, &state);
  entryinfo.update(&id, data);
  return 0;
}

static u32 fix_seqid_endianness(char seqid[]) {
  char id[] = {seqid[3], seqid[2], seqid[1], seqid[0]};
  return *((u32 *)id);
}

static int check_return_nfs4_run_open_task(struct pt_regs *ctx, u8 point_id) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  struct nfs4_opendata **odpp = nfs4_run_open_task_opendata.lookup(&id);
  if (odpp == NULL) {
    return 0;
  }
  struct nfs4_opendata *opendata = *odpp;

  add_data(data, point_id);
  copy_stateid(&data->run_open_task.return_o_res_stateid, &opendata->o_res.stateid);
  if (1 < fix_seqid_endianness(data->run_open_task.return_o_res_stateid.seqid)) {
    data->reason |= 1 << EFSS_OUTPUT_SEQID;
  }

  nfs4_run_open_task_opendata.delete(&id);
  entryinfo.update(&id, data);
  return 0;
}

static int check_update_open_stateid(struct pt_regs *ctx, u8 point_id,
                                     struct nfs4_state *state,
                                     const nfs4_stateid *open_stateid,
                                     const nfs4_stateid *delegation, fmode_t fmode) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  add_data(data, point_id);
  copy_stateid(&data->update_open_stateid.open_stateid, open_stateid);
  copy_state(&data->update_open_stateid.state, state);
  entryinfo.update(&id, data);
  return 0;
}

int enter__nfs4_atomic_open(struct pt_regs *ctx) { return check(ctx, 0); }

int enter__nfs4_client_recover_expired_lease(struct pt_regs *ctx) {
  return check(ctx, 1);
}
int enter__nfs4_wait_clnt_recover(struct pt_regs *ctx, struct nfs_client *clp) {
  return check__nfs4_wait_clnt_recover(ctx, 2, clp);
}
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

int enter__nfs4_opendata_alloc(struct pt_regs *ctx) { return check(ctx, 28); }
int return__nfs4_opendata_alloc(struct pt_regs *ctx) {
  return check_nfs4_opendata_alloc(ctx, 29);
}

int enter__nfs4_run_open_task(struct pt_regs *ctx, struct nfs4_opendata *data,
                              struct nfs_open_context *ctx2) {
  return check_enter_nfs4_run_open_task(ctx, 30, data);
}
int return__nfs4_run_open_task(struct pt_regs *ctx) {
  return check_return_nfs4_run_open_task(ctx, 31);
}

int enter___nfs4_proc_open_confirm(struct pt_regs *ctx) { return check(ctx, 34); }
int return___nfs4_proc_open_confirm(struct pt_regs *ctx) { return check(ctx, 35); }

int enter___nfs4_opendata_to_nfs4_state(struct pt_regs *ctx, struct nfs4_opendata *data) {
  return check__nfs4_opendata_to_nfs4_state(ctx, 32, data);
}
int return___nfs4_opendata_to_nfs4_state(struct pt_regs *ctx) { return check(ctx, 33); }

int enter__nfs4_get_open_state(struct pt_regs *ctx) { return check(ctx, 24); }
int enter____nfs4_find_state_byowner(struct pt_regs *ctx) { return check(ctx, 25); }
int return____nfs4_find_state_byowner(struct pt_regs *ctx) { return check(ctx, 26); }
int return__nfs4_get_open_state(struct pt_regs *ctx) { return check(ctx, 27); }

int enter__update_open_stateid(struct pt_regs *ctx, struct nfs4_state *state,
                               const nfs4_stateid *open_stateid,
                               const nfs4_stateid *delegation, fmode_t fmode) {
  return check_update_open_stateid(ctx, 7, state, open_stateid, delegation, fmode);
}
int enter__prepare_to_wait(struct pt_regs *ctx) { return check(ctx, 8); }
int return__prepare_to_wait(struct pt_regs *ctx) { return check(ctx, 9); }
int enter__finish_wait(struct pt_regs *ctx) { return check(ctx, 10); }
int return__finish_wait(struct pt_regs *ctx) { return check(ctx, 11); }
int enter__nfs4_schedule_state_manager(struct pt_regs *ctx) { return check(ctx, 20); }
int return__nfs4_schedule_state_manager(struct pt_regs *ctx) { return check(ctx, 21); }

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

int enter__nfs4_state_mark_reclaim_nograce(struct pt_regs *ctx, struct nfs_client *clp,
                                           struct nfs4_state *state) {
  return check_enter_nfs4_state_mark_reclaim_nograce(ctx, 22, state);
}
int return__nfs4_state_mark_reclaim_nograce(struct pt_regs *ctx) {
  return check_return_nfs4_state_mark_reclaim_nograce(ctx, 23);
}
