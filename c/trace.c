#include <linux/dcache.h>
#include <linux/fs.h>
#include <linux/nfs4.h>
#include <linux/nfs_fs.h>
#include <linux/sched.h>
#include <uapi/linux/nfs4.h>
#include <uapi/linux/ptrace.h>

#define SLOW_THRESHOLD_MS /*SLOW_THRESHOLD_MS*/
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

struct data_t {
  u64 ts;
  u8 point_ids[CALL_ORDER_COUNT];
  u32 point_deltas[CALL_ORDER_COUNT];
  u8 call_counts[SLOW_POINT_COUNT];
  char task[TASK_COMM_LEN];
  char file[DNAME_INLINE_LEN];
  u64 pid;
  u64 delta;
  char open_stateid_seqid[4];
  char open_stateid_other[NFS4_STATEID_OTHER_SIZE];
  char state_open_stateid_seqid[4];
  char state_open_stateid_other[NFS4_STATEID_OTHER_SIZE];
  char state_stateid_seqid[4];
  char state_stateid_other[NFS4_STATEID_OTHER_SIZE];
  char opendata_stateid_seqid[4];
  char opendata_stateid_other[NFS4_STATEID_OTHER_SIZE];
  char enter_runtask_stateid_seqid[4];
  char enter_runtask_stateid_other[NFS4_STATEID_OTHER_SIZE];
  char return_runtask_stateid_seqid[4];
  char return_runtask_stateid_other[NFS4_STATEID_OTHER_SIZE];
  u64 state_flags;
  u64 state_client_session;
  u32 open_stateid_type;
  u32 state_n_rdonly;
  u32 state_n_wronly;
  u32 state_n_rdwr;
  u32 state_state;
  u32 state_open_stateid_type;
  u32 state_stateid_type;
  u32 opendata_stateid_type;
  u32 enter_runtask_stateid_type;
  u32 return_runtask_stateid_type;
  u32 order_index;
  u32 show;
} __attribute__((__packed__));

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
  data->show = 0;
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
  // if (data->delta < SLOW_THRESHOLD_MS * 1000 && !data->show) {
  //   return 0;
  // }
  if (!(data->show || bpf_get_prandom_u32() % 1000 == 0)) {
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

static int check_show(struct pt_regs *ctx, u8 point_id) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  add_data(data, point_id);
  int ret = PT_REGS_RC(ctx);
  if (ret == 1) {
    data->show = 1;
  }

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
    bpf_probe_read_kernel(data->opendata_stateid_seqid, 4,
                          &opendata->o_res.stateid.seqid);
    bpf_probe_read_kernel(data->opendata_stateid_other, NFS4_STATEID_OTHER_SIZE,
                          opendata->o_res.stateid.data);
    bpf_probe_read_kernel(&data->opendata_stateid_type, sizeof(u32),
                          &opendata->o_res.stateid.type);
  }

  entryinfo.update(&id, data);
  return 0;
}

static int check_nfs4_run_open_task(struct pt_regs *ctx, u8 point_id) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  add_data(data, point_id);
  struct nfs4_opendata *opendata = (struct nfs4_opendata *)PT_REGS_RC(ctx);
  if (opendata != NULL) {
    bpf_probe_read_kernel(data->opendata_stateid_seqid, 4,
                          &opendata->o_res.stateid.seqid);
    bpf_probe_read_kernel(data->opendata_stateid_other, NFS4_STATEID_OTHER_SIZE,
                          opendata->o_res.stateid.data);
    bpf_probe_read_kernel(&data->opendata_stateid_type, sizeof(u32),
                          &opendata->o_res.stateid.type);
  }

  entryinfo.update(&id, data);
  return 0;
}

static int check_enter_nfs4_run_open_task(struct pt_regs *ctx, u8 point_id,
                                          struct nfs4_opendata *opendata) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  add_data(data, point_id);
  bpf_probe_read_kernel(data->enter_runtask_stateid_seqid, 4,
                        &opendata->o_res.stateid.seqid);
  bpf_probe_read_kernel(data->enter_runtask_stateid_other, NFS4_STATEID_OTHER_SIZE,
                        opendata->o_res.stateid.data);
  bpf_probe_read_kernel(&data->enter_runtask_stateid_type, sizeof(u32),
                        &opendata->o_res.stateid.type);

  entryinfo.update(&id, data);
  return 0;
}

static int check_return_nfs4_run_open_task(struct pt_regs *ctx, u8 point_id,
                                           struct nfs4_opendata *opendata) {
  u64 id = bpf_get_current_pid_tgid();
  struct data_t *data = entryinfo.lookup(&id);
  if (data == NULL) {
    return 0;
  }

  add_data(data, point_id);
  bpf_probe_read_kernel(data->return_runtask_stateid_seqid, 4,
                        &opendata->o_res.stateid.seqid);
  bpf_probe_read_kernel(data->return_runtask_stateid_other, NFS4_STATEID_OTHER_SIZE,
                        opendata->o_res.stateid.data);
  bpf_probe_read_kernel(&data->return_runtask_stateid_type, sizeof(u32),
                        &opendata->o_res.stateid.type);

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
  bpf_probe_read_kernel(data->open_stateid_seqid, 4, &open_stateid->seqid);
  bpf_probe_read_kernel(data->open_stateid_other, NFS4_STATEID_OTHER_SIZE,
                        open_stateid->other);
  bpf_probe_read_kernel(data->state_open_stateid_seqid, 4, &state->open_stateid.seqid);
  bpf_probe_read_kernel(data->state_open_stateid_other, NFS4_STATEID_OTHER_SIZE,
                        state->open_stateid.data);
  bpf_probe_read_kernel(data->state_stateid_seqid, 4, &state->stateid.seqid);
  bpf_probe_read_kernel(data->state_stateid_other, NFS4_STATEID_OTHER_SIZE,
                        state->stateid.other);
  bpf_probe_read_kernel(&data->state_flags, sizeof(u64), &state->flags);
  bpf_probe_read_kernel(&data->open_stateid_type, sizeof(u32), &open_stateid->type);
  bpf_probe_read_kernel(&data->state_n_rdwr, sizeof(u32), &state->n_rdwr);
  bpf_probe_read_kernel(&data->state_n_wronly, sizeof(u32), &state->n_wronly);
  bpf_probe_read_kernel(&data->state_n_rdonly, sizeof(u32), &state->n_rdonly);
  bpf_probe_read_kernel(&data->state_state, sizeof(u32), &state->state);
  bpf_probe_read_kernel(&data->state_open_stateid_type, sizeof(u32),
                        &state->open_stateid.type);
  bpf_probe_read_kernel(&data->state_stateid_type, sizeof(u32), &state->stateid.type);
  bpf_probe_read_kernel(
      &data->state_client_session, sizeof(u64),
      &((struct nfs_server *)(state->inode->i_sb->s_fs_info))->nfs_client->cl_session);
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

int enter__nfs4_opendata_alloc(struct pt_regs *ctx) { return check(ctx, 28); }
int return__nfs4_opendata_alloc(struct pt_regs *ctx) {
  return check_nfs4_opendata_alloc(ctx, 29);
}

int enter__nfs4_run_open_task(struct pt_regs *ctx, struct nfs4_opendata *data,
                              struct nfs_open_context *ctx2) {
  return check_enter_nfs4_run_open_task(ctx, 30, data);
}
int return__nfs4_run_open_task(struct pt_regs *ctx, struct nfs4_opendata *data,
                               struct nfs_open_context *ctx2) {
  return check_return_nfs4_run_open_task(ctx, 31, data);
}

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

int enter__nfs4_state_mark_reclaim_nograce(struct pt_regs *ctx) { return check(ctx, 22); }
int return__nfs4_state_mark_reclaim_nograce(struct pt_regs *ctx) {
  return check_show(ctx, 23);
}
