package notify

import "github.com/gen2brain/beeep"

// Send shows a native Windows toast notification.
func Send(title, message string) {
	_ = beeep.Notify(title, message, "")
}
