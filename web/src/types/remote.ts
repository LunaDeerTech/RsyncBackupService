export interface RemoteConfig {
  id: number
  name: string
  type: 'ssh' | 'openlist' | 'cloud'
  host: string
  port: number
  username: string
  cloud_provider?: string
  created_at: string
  updated_at: string
}
