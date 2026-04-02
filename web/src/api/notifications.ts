import { apiFetch } from "./client"
import type {
	NotificationChannelPayload,
	NotificationChannelSummary,
	NotificationSubscription,
	NotificationSubscriptionPayload,
	JsonValue,
} from "./types"

export async function listNotificationChannels(): Promise<NotificationChannelSummary[]> {
	return apiFetch<NotificationChannelSummary[]>("/api/notification-channels")
}

export async function createNotificationChannel(payload: NotificationChannelPayload): Promise<NotificationChannelSummary> {
	return apiFetch<NotificationChannelSummary>("/api/notification-channels", {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function updateNotificationChannel(id: number, payload: NotificationChannelPayload): Promise<NotificationChannelSummary> {
	return apiFetch<NotificationChannelSummary>(`/api/notification-channels/${id}`, {
		method: "PUT",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function deleteNotificationChannel(id: number): Promise<void> {
	await apiFetch<void>(`/api/notification-channels/${id}`, {
		method: "DELETE",
	})
}

export async function testNotificationChannel(id: number, channelConfig: JsonValue): Promise<void> {
	await apiFetch<void>(`/api/notification-channels/${id}/test`, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify({
			channel_config: channelConfig,
		}),
	})
}

export async function listSubscriptions(instanceId: number): Promise<NotificationSubscription[]> {
	return apiFetch<NotificationSubscription[]>(`/api/instances/${instanceId}/subscriptions`)
}

export async function upsertSubscription(instanceId: number, payload: NotificationSubscriptionPayload): Promise<NotificationSubscription> {
	return apiFetch<NotificationSubscription>(`/api/instances/${instanceId}/subscriptions`, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function deleteSubscription(id: number): Promise<void> {
	await apiFetch<void>(`/api/subscriptions/${id}`, {
		method: "DELETE",
	})
}