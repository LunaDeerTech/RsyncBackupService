export interface ApiTokenPair {
	access_token: string
	refresh_token: string
}

export interface LoginResponse extends ApiTokenPair {}

export interface VerifyResponse {
	verify_token: string
}

export interface SessionTokens {
	accessToken: string
	refreshToken: string
}

export interface LoginCredentials {
	username: string
	password: string
}

export interface AuthUser {
	id: number
	username: string
	is_admin: boolean
	created_at: string
	updated_at: string
}

export interface ApiErrorResponse {
	error?: string
}

export type JsonValue = null | boolean | number | string | JsonValue[] | { [key: string]: JsonValue }

export type BackupStatus = "running" | "success" | "failed" | "cancelled"
export type BackupType = "rolling" | "cold"
export type SourceType = "local" | "remote"
export type InstanceRole = "admin" | "viewer"

export interface InstanceBase {
	id: number
	name: string
	source_type: SourceType | string
	source_host?: string
	source_port: number
	source_user?: string
	source_ssh_key_id?: number | null
	source_path: string
	exclude_patterns: string[]
	enabled: boolean
	created_by: number
	created_at: string
	updated_at: string
}

export interface InstanceSummary extends InstanceBase {
	strategy_count: number
	last_backup_status?: BackupStatus | string
	last_backup_at?: string | null
	relay_mode: boolean
	relay_mode_uncertain: boolean
}

export type InstanceDetail = InstanceBase

export interface CreateInstancePayload {
	name: string
	source_type: SourceType | string
	source_host?: string
	source_port?: number
	source_user?: string
	source_ssh_key_id?: number | null
	source_path: string
	exclude_patterns?: string[]
	enabled: boolean
}

export type UpdateInstancePayload = CreateInstancePayload

export interface StrategySummary {
	id: number
	instance_id: number
	name: string
	backup_type: BackupType | string
	cron_expr?: string | null
	interval_seconds: number
	retention_days: number
	retention_count: number
	cold_volume_size?: string | null
	max_execution_seconds: number
	storage_target_ids: number[]
	enabled: boolean
	created_at: string
	updated_at: string
}

export interface StrategyPayload {
	name: string
	backup_type: BackupType | string
	cron_expr?: string | null
	interval_seconds: number
	retention_days: number
	retention_count: number
	cold_volume_size?: string | null
	max_execution_seconds: number
	storage_target_ids: number[]
	enabled: boolean
}

export interface StorageTargetSummary {
	id: number
	name: string
	type: string
	host?: string
	port: number
	user?: string
	ssh_key_id?: number | null
	base_path: string
	created_at: string
	updated_at: string
}

export interface StorageTargetPayload {
	name: string
	type: string
	host?: string
	port?: number
	user?: string
	ssh_key_id?: number | null
	base_path: string
}

export interface SSHKeySummary {
	id: number
	name: string
	fingerprint: string
	created_at: string
}

export interface SSHKeyPayload {
	name: string
	private_key_path: string
}

export interface SSHKeyConnectionPayload {
	host: string
	port?: number
	user: string
}

export interface BackupRecord {
	id: number
	instance_id: number
	storage_target_id: number
	strategy_id?: number | null
	backup_type: BackupType | string
	status: BackupStatus | string
	snapshot_path: string
	bytes_transferred: number
	files_transferred: number
	total_size: number
	volume_count: number
	started_at: string
	finished_at?: string | null
	error_message?: string
}

export interface RestoreRecord {
	id: number
	instance_id: number
	backup_record_id: number
	restore_target_path: string
	overwrite: boolean
	status: BackupStatus | string
	started_at: string
	finished_at?: string | null
	error_message?: string
	triggered_by: number
}

export interface RestorePayload {
	instance_id: number
	backup_record_id: number
	restore_target_path: string
	overwrite: boolean
	verify_token: string
}

export interface NotificationChannelSummary {
	id: number
	name: string
	type: string
	enabled: boolean
	config?: JsonValue
	created_at?: string
	updated_at?: string
}

export interface NotificationChannelPayload {
	name: string
	type: string
	config: JsonValue
	enabled: boolean
}

export interface NotificationSubscription {
	id: number
	user_id: number
	instance_id: number
	channel_id: number
	channel: NotificationChannelSummary
	events: string[]
	channel_config: JsonValue
	enabled: boolean
	created_at: string
}

export interface NotificationSubscriptionPayload {
	channel_id: number
	events: string[]
	channel_config: JsonValue
	enabled: boolean
}

export interface AuditLogItem {
	id: number
	user_id: number
	username: string
	action: string
	resource_type: string
	resource_id: number
	detail: JsonValue
	ip_address: string
	created_at: string
}

export interface AuditLogListResponse {
	items: AuditLogItem[]
	total: number
	page: number
	page_size: number
}

export interface AuditLogQuery {
	action?: string
	resource_type?: string
	user_id?: number
	start_time?: string
	end_time?: string
	page?: number
	page_size?: number
}

export interface RunningTaskStatus {
	task_id: string
	instance_id: number
	storage_target_id: number
	started_at: string
	percentage: number
	speed_text: string
	remaining_text: string
	status: BackupStatus | string
}

export interface ProgressEvent {
	task_id: string
	instance_id: number
	percentage: number
	speed_text: string
	remaining_text: string
	status: BackupStatus | string
}

export interface DashboardBackupSummary {
	id: number
	instance_id: number
	instance_name: string
	storage_target_id: number
	backup_type: BackupType | string
	status: BackupStatus | string
	started_at: string
	finished_at?: string | null
}

export interface DashboardStorageSummary {
	storage_target_id: number
	storage_target_name: string
	storage_target_type: string
	available_bytes: number
	backup_count: number
	last_backup_at?: string | null
}

export interface DashboardSummary {
	instance_count: number
	today_backup_count: number
	success_count: number
	failed_count: number
	running_tasks: RunningTaskStatus[]
	recent_backups: DashboardBackupSummary[]
	storage_overview: DashboardStorageSummary[]
}

export interface SystemStatus {
	version: string
	data_dir: string
	uptime_seconds: number
	disk_total_bytes: number
	disk_free_bytes: number
}

export interface UserSummary {
	id: number
	username: string
	is_admin: boolean
	created_at: string
	updated_at: string
}

export interface CreateUserPayload {
	username: string
	password: string
	is_admin: boolean
}

export interface ResetUserPasswordPayload {
	password: string
	verify_token: string
}

export interface InstancePermission {
	user_id: number
	username: string
	instance_id: number
	role: InstanceRole | string
}

export interface PermissionPayload {
	user_id: number
	role: InstanceRole | string
}

export function normalizeTokenPair(tokens: ApiTokenPair): SessionTokens {
	return {
		accessToken: tokens.access_token,
		refreshToken: tokens.refresh_token,
	}
}