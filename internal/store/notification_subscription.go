package store

import (
	"fmt"
	"sort"
	"time"

	"rsync-backup-service/internal/model"
)

const notificationSubscriptionColumns = `ns.id, ns.user_id, ns.instance_id, i.name, ns.enabled, ns.created_at`

type notificationSubscriptionScanner interface {
	Scan(dest ...any) error
}

func (db *DB) ListSubscriptionsByUser(userID int64) ([]model.NotificationSubscription, error) {
	rows, err := db.Query(
		`SELECT `+notificationSubscriptionColumns+`
		 FROM notification_subscriptions ns
		 JOIN instances i ON i.id = ns.instance_id
		 WHERE ns.user_id = ?
		 ORDER BY ns.instance_id ASC, ns.id ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions by user %d: %w", userID, err)
	}
	defer rows.Close()

	subscriptions := make([]model.NotificationSubscription, 0)
	for rows.Next() {
		subscription, err := scanNotificationSubscription(rows)
		if err != nil {
			return nil, fmt.Errorf("scan subscription by user %d: %w", userID, err)
		}
		subscriptions = append(subscriptions, *subscription)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscriptions by user %d: %w", userID, err)
	}

	return subscriptions, nil
}

func (db *DB) UpdateSubscriptions(userID int64, subs []model.NotificationSubscription) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin update subscriptions for user %d: %w", userID, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM notification_subscriptions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("clear subscriptions for user %d: %w", userID, err)
	}

	sorted := make([]model.NotificationSubscription, 0, len(subs))
	for _, sub := range subs {
		if sub.InstanceID <= 0 {
			return fmt.Errorf("subscription instance id is required")
		}
		sorted = append(sorted, model.NotificationSubscription{InstanceID: sub.InstanceID, Enabled: sub.Enabled})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].InstanceID < sorted[j].InstanceID
	})

	for _, sub := range sorted {
		if !sub.Enabled {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO notification_subscriptions (user_id, instance_id, enabled, created_at)
			 VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
			userID,
			sub.InstanceID,
			true,
		); err != nil {
			return fmt.Errorf("insert subscription user %d instance %d: %w", userID, sub.InstanceID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit subscriptions for user %d: %w", userID, err)
	}

	return nil
}

func (db *DB) ListSubscribersByInstance(instanceID int64) ([]model.User, error) {
	rows, err := db.Query(
		`SELECT DISTINCT u.id, u.email, u.name, u.password_hash, u.role, u.created_at, u.updated_at
		 FROM users u
		 JOIN notification_subscriptions ns ON ns.user_id = u.id
		 WHERE ns.instance_id = ? AND ns.enabled = 1
		 ORDER BY u.id ASC`,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list subscribers by instance %d: %w", instanceID, err)
	}
	defer rows.Close()

	users := make([]model.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan subscriber by instance %d: %w", instanceID, err)
		}
		users = append(users, *user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscribers by instance %d: %w", instanceID, err)
	}

	return users, nil
}

func scanNotificationSubscription(scanner notificationSubscriptionScanner) (*model.NotificationSubscription, error) {
	var (
		subscription model.NotificationSubscription
		rawCreated   string
	)

	if err := scanner.Scan(
		&subscription.ID,
		&subscription.UserID,
		&subscription.InstanceID,
		&subscription.InstanceName,
		&subscription.Enabled,
		&rawCreated,
	); err != nil {
		return nil, err
	}

	createdAt, err := parseSQLiteTime(rawCreated)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", rawCreated, err)
	}
	subscription.CreatedAt = createdAt
	return &subscription, nil
}

func mergeSubscriptionsWithInstances(instances []model.Instance, current []model.NotificationSubscription, userID int64) []model.NotificationSubscription {
	enabledByInstance := make(map[int64]model.NotificationSubscription, len(current))
	for _, subscription := range current {
		enabledByInstance[subscription.InstanceID] = subscription
	}

	merged := make([]model.NotificationSubscription, 0, len(instances))
	for _, instance := range instances {
		subscription := model.NotificationSubscription{
			UserID:       userID,
			InstanceID:   instance.ID,
			InstanceName: instance.Name,
			Enabled:      false,
			CreatedAt:    time.Time{},
		}
		if currentSub, ok := enabledByInstance[instance.ID]; ok {
			subscription.ID = currentSub.ID
			subscription.Enabled = currentSub.Enabled
			subscription.CreatedAt = currentSub.CreatedAt
		}
		merged = append(merged, subscription)
	}

	return merged
}