package components

import (
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"bbrew/internal/ui/theme"
)

// MessageMinDuration is how long a status message stays visible before a
// queued message may replace it.
const MessageMinDuration = 3 * time.Second

// maxPendingMessages bounds the queue so a burst of progress messages does not
// lag far behind; the oldest pending messages are dropped first.
const maxPendingMessages = 5

type statusMessage struct {
	text  string
	color tcell.Color
}

// Notifier shows status messages one at a time. Each message stays for at
// least MessageMinDuration; messages arriving sooner are queued. The last
// message stays until another one replaces it.
type Notifier struct {
	view  *tview.TextView
	theme *theme.Theme

	mu        sync.Mutex
	queue     []statusMessage
	shownAt   time.Time
	timer     *time.Timer
	scheduler func(func()) // runs a function on the UI goroutine and redraws
}

func NewNotifier(theme *theme.Theme) *Notifier {
	notifierView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	return &Notifier{
		view:  notifierView,
		theme: theme,
	}
}

// SetScheduler sets how queued messages get displayed later from a timer,
// typically a wrapper around tview.Application.QueueUpdateDraw.
func (n *Notifier) SetScheduler(scheduler func(func())) {
	n.scheduler = scheduler
}

func (n *Notifier) View() *tview.TextView {
	return n.view
}

// ShowInfo shows a neutral message.
func (n *Notifier) ShowInfo(message string) {
	n.push(message, n.theme.DefaultTextColor)
}

func (n *Notifier) ShowSuccess(message string) {
	n.push(message, n.theme.SuccessColor)
}

func (n *Notifier) ShowWarning(message string) {
	n.push(message, n.theme.WarningColor)
}

func (n *Notifier) ShowError(message string) {
	n.push(message, n.theme.ErrorColor)
}

func (n *Notifier) Clear() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.queue = nil
	n.view.Clear()
}

func (n *Notifier) push(text string, color tcell.Color) {
	n.mu.Lock()
	defer n.mu.Unlock()

	msg := statusMessage{text: text, color: color}
	if n.scheduler == nil || (len(n.queue) == 0 && time.Since(n.shownAt) >= MessageMinDuration) {
		n.display(msg)
		return
	}

	n.queue = append(n.queue, msg)
	if len(n.queue) > maxPendingMessages {
		n.queue = n.queue[len(n.queue)-maxPendingMessages:]
	}
	n.armTimer()
}

// display shows a message now. Caller holds n.mu.
func (n *Notifier) display(msg statusMessage) {
	n.shownAt = time.Now()
	n.view.SetTextColor(msg.color).SetText(" " + tview.Escape(msg.text) + " ")
}

// armTimer schedules the next queued message. Caller holds n.mu.
func (n *Notifier) armTimer() {
	if n.timer != nil {
		return
	}
	wait := max(MessageMinDuration-time.Since(n.shownAt), 0)
	n.timer = time.AfterFunc(wait, func() {
		n.scheduler(n.next)
	})
}

// next shows the next queued message; runs on the UI goroutine.
func (n *Notifier) next() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.timer = nil
	if len(n.queue) == 0 {
		return
	}
	n.display(n.queue[0])
	n.queue = n.queue[1:]
	if len(n.queue) > 0 {
		n.armTimer()
	}
}
