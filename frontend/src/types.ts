export type Device = {
  device_type: string;
  device_subtype: string;
  is_connected: boolean;
  name: string;
  path: string;
};

export type Subtask = {
  id: string;
  schedule_type: string;
  status: string;
  progress: number;
  speed: number;
  total_files: number;
  xferred_files: number;
  total_size: number;
  xferred_size: number;
  updated_at: number;
  error_message: string;
  target: Device;
};

export type ComposeBackupHook = "off" | "before" | "on_success" | "both";

export type TaskPrefs = {
  task_id: string;
  enabled: boolean;
  notify_enabled: boolean;
  notify_success: boolean;
  notify_failure: boolean;
  compose_backup_hook?: ComposeBackupHook;
};

export type Task = {
  id: string;
  name: string;
  status: string;
  last_backup_time: number;
  source: Device;
  subtasks: Subtask[];
  prefs: TaskPrefs;
};

export type BackupRun = {
  id: number;
  task_id: string;
  backup_name: string;
  source: Device;
  target: Device;
  started_at: string;
  ended_at?: string;
  duration_ms?: number;
  duration_quality: string;
  result: string;
  total_files: number;
  xferred_files: number;
  total_size: number;
  xferred_size: number;
  error_message: string;
  discord_status: string;
};

export type Schedule = {
  id: number;
  task_id: string;
  name: string;
  cron_expr: string;
  timezone: string;
  enabled: boolean;
  last_triggered_at?: string;
};

export type Settings = {
  language: string;
  theme: string;
  timezone: string;
  zimaos_api_base_url: string;
  zimaos_api_token_set: boolean;
  zimaos_username: string;
  zimaos_password_set: boolean;
  poll_interval_seconds: number;
  discord_enabled: boolean;
  discord_webhook_set: boolean;
  discord_notify_success: boolean;
  discord_notify_failure: boolean;
  setup_completed: boolean;
  compose_backup_enabled: boolean;
  compose_backup_cron: string;
  compose_backup_keep: number;
  compose_backup_appdata_dir?: string;
  compose_backup_appdata_host_root?: string;
  compose_backup_compose_dir?: string;
  compose_backup_compose_host_root?: string;
  compose_backup_docker_sock?: string;
  compose_backup_last_ok?: string;
  compose_backup_last_error?: string;
  compose_backup_last_file?: string;
  compose_backup_last_apps?: number;
  compose_backup_last_fail?: number;
};

export type ComposeProject = {
  name: string;
  compose_host: string;
  compose_local: string;
  appdata_name: string;
  appdata_local: string;
  containers: string[];
  compose_readable: boolean;
  appdata_writable: boolean;
};

export type ComposeBackupStatus = {
  enabled: boolean;
  docker_available: boolean;
  docker_error?: string | null;
  docker_sock: string;
  appdata_dir: string;
  appdata_available: boolean;
  appdata_host_root: string;
  compose_dir: string;
  compose_host_root: string;
  projects: ComposeProject[];
  project_count: number;
  cron: string;
  last_ok_at?: string | null;
  last_error?: string | null;
  last_ok_count: number;
  last_fail_count: number;
};
