package bench

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strconv"
)

// Flag defaults. java/bench/Config.java uses the same values. The Scenario
// sets the channel, message length and fragment limit defaults.
const (
	DefaultStreamID       = 1001
	DefaultPongStreamID   = 1002
	DefaultMessageLength  = 32
	DefaultMessages       = 1_000_000
	DefaultWarmupMessages = 100_000
	DefaultPublication    = ConcurrentPublication
	DefaultSendMethod     = SendOffer
	DefaultClusterMembers = 1

	// DefaultRate and DefaultWarmupRate are the send rates of a fixed-rate
	// scenario in messages per second. They match the Aeron load test rig.
	DefaultRate       = 501_000
	DefaultWarmupRate = 25_000
)

// MinMessageLength is the shortest message. Each message starts with an
// 8-byte sequence number or timestamp.
const MinMessageLength = 8

// MinFixedRateMessageLength is the shortest fixed-rate message. Each
// fixed-rate message starts with an 8-byte scheduled send time and an
// 8-byte sequence number.
const MinFixedRateMessageLength = 16

// MaxRate is the highest fixed-rate send rate in messages per second. The
// send interval is then 1 ns.
const MaxRate = 1_000_000_000

// Publication kinds for the -publication flag.
const (
	ConcurrentPublication = "concurrent"
	ExclusivePublication  = "exclusive"
)

// Send methods for the -send-method flag.
const (
	SendOffer    = "offer"
	SendTryClaim = "try-claim"
)

// Config holds the flags of a scenario. Each client parses the same flag
// names, except -unlock-threads, which only Luas takes. Only a cluster
// scenario takes -cluster-members. Only a fixed-rate scenario takes -rate
// and -warmup-rate.
type Config struct {
	AeronDir string `json:"aeron_dir"`
	Channel  string `json:"channel"`
	StreamID int32  `json:"stream_id"`

	// PongStreamID is the stream of the ping-pong replies. The throughput
	// scenarios do not use it. Validate still rejects it when it equals
	// StreamID, in every scenario.
	PongStreamID int32 `json:"pong_stream_id"`

	MessageLength  int   `json:"message_length"`
	Messages       int64 `json:"messages"`
	WarmupMessages int64 `json:"warmup_messages"`
	FragmentLimit  int   `json:"fragment_limit"`

	// Publication holds ConcurrentPublication or ExclusivePublication.
	Publication string `json:"publication"`

	// SendMethod holds SendOffer or SendTryClaim.
	SendMethod string `json:"send_method"`

	// ClusterMembers is the member count of the cluster that
	// cluster-ping-pong runs against. 0 in every other scenario.
	ClusterMembers int `json:"cluster_members,omitempty"`

	// Rate is the send rate of a fixed-rate scenario in messages per second.
	// 0 in every other scenario.
	Rate int64 `json:"rate,omitempty"`

	// WarmupRate is the warmup send rate of a fixed-rate scenario in
	// messages per second. 0 in every other scenario.
	WarmupRate int64 `json:"warmup_rate,omitempty"`

	// CPUs holds one CPU per hot thread. PinHotThread binds hot thread i to
	// CPUs[i]. An empty list leaves every thread unpinned. ParseFlags returns
	// an empty list, not nil, so the JSON matches Java.
	CPUs []int `json:"cpus"`

	// UnlockThreads makes a hot goroutine unlock after its pin. PinHotThread
	// unlocks before it returns. RunPinnedPair unlocks after both pins have
	// returned. The thread keeps its CPU set. The runtime may then move the
	// goroutine to another thread.
	UnlockThreads bool `json:"unlock_threads,omitempty"`

	// YieldIntervalNs is the time between two voluntary yields of one
	// unlocked hot goroutine. ParseFlagsWithUnlock sets
	// DefaultYieldIntervalNs with UnlockThreads. 0 never yields. See
	// HotYield.
	YieldIntervalNs int64 `json:"yield_interval_ns,omitempty"`

	// Output names the result JSON file. An empty Output writes to stdout.
	Output string `json:"-"`
}

// ParseFlags parses args, the arguments after the scenario name. Flag
// defaults: the Default constants, the scenario defaults, and AERON_DIR from
// the environment. Integer flags take decimal values only, as in Java. Only
// a cluster scenario takes -cluster-members. Only a fixed-rate scenario
// takes -rate and -warmup-rate. ParseFlags rejects -unlock-threads.
//
// Returns an error when:
//   - args hold -h or -help (flag.ErrHelp)
//   - a flag does not parse
//   - a positional argument follows the flags
//   - Validate fails
//   - a cluster scenario fails ValidateCluster
//   - a fixed-rate scenario fails ValidateFixedRate
func ParseFlags(scenario Scenario, args []string) (Config, error) {
	return parseFlags(scenario, args, false)
}

// ParseFlagsWithUnlock parses like ParseFlags. ParseFlagsWithUnlock also
// takes -unlock-threads. It then sets DefaultYieldIntervalNs.
func ParseFlagsWithUnlock(scenario Scenario, args []string) (Config, error) {
	return parseFlags(scenario, args, true)
}

func parseFlags(scenario Scenario, args []string, canUnlock bool) (Config, error) {
	cfg := Config{
		AeronDir:       os.Getenv("AERON_DIR"),
		Channel:        scenario.Channel,
		StreamID:       DefaultStreamID,
		PongStreamID:   DefaultPongStreamID,
		MessageLength:  scenario.MessageLength,
		Messages:       DefaultMessages,
		WarmupMessages: DefaultWarmupMessages,
		FragmentLimit:  scenario.FragmentLimit,
		Publication:    DefaultPublication,
		SendMethod:     DefaultSendMethod,
		CPUs:           []int{},
	}
	flags := flag.NewFlagSet(scenario.Name, flag.ContinueOnError)
	flags.StringVar(&cfg.AeronDir, "aeron-dir", cfg.AeronDir, "driver directory")
	flags.StringVar(&cfg.Channel, "channel", cfg.Channel, "channel")
	flags.Var(decimalFlag[int32]{&cfg.StreamID, 32}, "stream-id", "stream id")
	flags.Var(decimalFlag[int32]{&cfg.PongStreamID, 32}, "pong-stream-id", "ping-pong reply stream id")
	flags.Var(decimalFlag[int]{&cfg.MessageLength, 32}, "message-length", "message length in bytes")
	flags.Var(decimalFlag[int64]{&cfg.Messages, 64}, "messages", "measured message count")
	flags.Var(decimalFlag[int64]{&cfg.WarmupMessages, 64}, "warmup-messages", "warmup message count")
	flags.Var(decimalFlag[int]{&cfg.FragmentLimit, 32}, "fragment-limit", "fragment limit per poll")
	flags.StringVar(
		&cfg.Publication,
		"publication",
		cfg.Publication,
		"publication kind: concurrent or exclusive",
	)
	flags.StringVar(&cfg.SendMethod, "send-method", cfg.SendMethod, "send method: offer or try-claim")
	if scenario.IsCluster {
		cfg.ClusterMembers = DefaultClusterMembers
		flags.Var(decimalFlag[int]{&cfg.ClusterMembers, 32}, "cluster-members", "cluster member count")
	}
	if scenario.IsFixedRate {
		cfg.Rate = DefaultRate
		cfg.WarmupRate = DefaultWarmupRate
		flags.Var(decimalFlag[int64]{&cfg.Rate, 64}, "rate", "send rate in messages per second")
		flags.Var(
			decimalFlag[int64]{&cfg.WarmupRate, 64},
			"warmup-rate",
			"warmup send rate in messages per second",
		)
	}
	flags.Func(
		"cpus",
		"CPU list, one CPU per hot thread (default: no pinning)",
		func(text string) error {
			cpus, err := ParseCPUList(text)
			cfg.CPUs = cpus
			return err
		},
	)
	if canUnlock {
		flags.BoolVar(
			&cfg.UnlockThreads,
			"unlock-threads",
			false,
			"unlock each hot goroutine after pinning its thread",
		)
	}
	flags.StringVar(&cfg.Output, "output", "", "result JSON file (default: stdout)")
	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	if cfg.UnlockThreads {
		cfg.YieldIntervalNs = DefaultYieldIntervalNs
	}
	if flags.NArg() > 0 {
		return Config{}, fmt.Errorf("parse flags: unexpected argument %q", flags.Arg(0))
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	if scenario.IsCluster {
		if err := cfg.ValidateCluster(); err != nil {
			return Config{}, err
		}
	}
	if scenario.IsFixedRate {
		if err := cfg.ValidateFixedRate(); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

// decimalFlag is a flag.Value for a decimal integer of bitSize bits. For
// ASCII text, decimalFlag accepts what Java Integer.parseInt and
// Long.parseLong accept.
type decimalFlag[T int | int32 | int64] struct {
	target  *T
	bitSize int
}

func (f decimalFlag[T]) String() string {
	if f.target == nil {
		return "0"
	}
	return strconv.FormatInt(int64(*f.target), 10)
}

func (f decimalFlag[T]) Set(text string) error {
	value, err := strconv.ParseInt(text, 10, f.bitSize)
	if errors.Is(err, strconv.ErrRange) {
		return fmt.Errorf("outside int%d", f.bitSize)
	}
	if err != nil {
		return errors.New("want a decimal integer")
	}
	*f.target = T(value)
	return nil
}

// Validate checks the flag values.
//
// Returns an error when:
//   - AeronDir or Channel is empty
//   - PongStreamID equals StreamID
//   - MessageLength is below MinMessageLength
//   - Messages or FragmentLimit is below 1
//   - WarmupMessages is below 0
//   - Publication or SendMethod does not name a known value
func (c Config) Validate() error {
	var errs []error
	if c.AeronDir == "" {
		errs = append(errs, errors.New("aeron dir empty: set -aeron-dir or AERON_DIR"))
	}
	if c.Channel == "" {
		errs = append(errs, errors.New("channel empty"))
	}
	if c.PongStreamID == c.StreamID {
		errs = append(errs, fmt.Errorf("pong stream id %d equals stream id", c.PongStreamID))
	}
	if c.MessageLength < MinMessageLength {
		errs = append(errs, fmt.Errorf("message length %d below %d", c.MessageLength, MinMessageLength))
	}
	if c.Messages < 1 {
		errs = append(errs, fmt.Errorf("messages %d below 1", c.Messages))
	}
	if c.WarmupMessages < 0 {
		errs = append(errs, fmt.Errorf("warmup messages %d below 0", c.WarmupMessages))
	}
	if c.FragmentLimit < 1 {
		errs = append(errs, fmt.Errorf("fragment limit %d below 1", c.FragmentLimit))
	}
	if !slices.Contains([]string{ConcurrentPublication, ExclusivePublication}, c.Publication) {
		errs = append(
			errs,
			fmt.Errorf(
				"publication %q: want %s or %s",
				c.Publication,
				ConcurrentPublication,
				ExclusivePublication,
			),
		)
	}
	if !slices.Contains([]string{SendOffer, SendTryClaim}, c.SendMethod) {
		errs = append(
			errs,
			fmt.Errorf("send method %q: want %s or %s", c.SendMethod, SendOffer, SendTryClaim),
		)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return nil
}

// ValidateCluster checks the limits that every cluster client shares.
//
// Returns an error when:
//   - ClusterMembers is below 1
//   - StreamID is not DefaultStreamID: java/bench/ClusterServer.java fixes
//     the ingress stream at DefaultStreamID
//   - SendMethod is not SendOffer: aeron-go AeronCluster does not have TryClaim
//   - FragmentLimit is not ClusterEgressFragmentLimit
func (c Config) ValidateCluster() error {
	var errs []error
	if c.ClusterMembers < 1 {
		errs = append(errs, fmt.Errorf("cluster members %d below 1", c.ClusterMembers))
	}
	if c.StreamID != DefaultStreamID {
		errs = append(errs, fmt.Errorf("stream id %d: the cluster takes ingress on stream %d only",
			c.StreamID, DefaultStreamID))
	}
	if c.SendMethod != SendOffer {
		errs = append(
			errs,
			fmt.Errorf("send method %q: cluster ingress takes %s only", c.SendMethod, SendOffer),
		)
	}
	if c.FragmentLimit != ClusterEgressFragmentLimit {
		errs = append(
			errs,
			fmt.Errorf("fragment limit %d: cluster clients poll egress %d fragments at a time",
				c.FragmentLimit, ClusterEgressFragmentLimit),
		)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("invalid cluster config: %w", err)
	}
	return nil
}

// ValidateFixedRate checks the limits of a fixed-rate scenario.
//
// Returns an error when:
//   - Rate or WarmupRate is below 1 or above MaxRate
//   - MessageLength is below MinFixedRateMessageLength
func (c Config) ValidateFixedRate() error {
	var errs []error
	if c.Rate < 1 || c.Rate > MaxRate {
		errs = append(errs, fmt.Errorf("rate %d: want 1 to %d", c.Rate, MaxRate))
	}
	if c.WarmupRate < 1 || c.WarmupRate > MaxRate {
		errs = append(errs, fmt.Errorf("warmup rate %d: want 1 to %d", c.WarmupRate, MaxRate))
	}
	if c.MessageLength < MinFixedRateMessageLength {
		errs = append(
			errs,
			fmt.Errorf("message length %d below %d", c.MessageLength, MinFixedRateMessageLength),
		)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("invalid fixed-rate config: %w", err)
	}
	return nil
}

// Variant returns the result variant label: publication kind, send method
// and message length, for example "exclusive-try-claim-256". A cluster
// config adds the member count, for example "exclusive-offer-32-members-3".
// A fixed-rate config adds the rate, for example
// "exclusive-try-claim-32-rate-501000".
func (c Config) Variant() string {
	variant := c.Publication + "-" + c.SendMethod + "-" + strconv.Itoa(c.MessageLength)
	if c.ClusterMembers > 0 {
		variant += "-members-" + strconv.Itoa(c.ClusterMembers)
	}
	if c.Rate > 0 {
		variant += "-rate-" + strconv.FormatInt(c.Rate, 10)
	}
	return variant
}

// IsFragmented reports whether MessageLength exceeds MaxPayloadLength. The
// throughput subscriber reassembles each message. Fixed-rate ping and pong
// do the same, because each side has to see the whole message.
func (c Config) IsFragmented() bool {
	return c.MessageLength > MaxPayloadLength
}

// PinHotThread binds the calling goroutine to CPUs[index] through PinThread.
// With UnlockThreads, PinHotThread then unlocks the goroutine. PinHotThread
// does nothing when CPUs is empty.
//
// Returns an error when CPUs is not empty and does not have an entry at
// index, or when PinThread fails.
func (c Config) PinHotThread(index int) error {
	if len(c.CPUs) == 0 {
		return nil
	}
	if err := c.pinLocked(index); err != nil {
		return err
	}
	c.unlockHot()
	return nil
}

// RunPinnedPair pins the calling goroutine to CPUs[first]. It starts fn on a
// new goroutine. It waits until that goroutine has pinned itself to
// CPUs[second]. Both goroutines stay locked through the wait. When
// UnlockThreads is set, RunPinnedPair then unlocks each one. fn runs after
// its own unlock.
//
// RunPinnedPair starts fn and skips the pins when CPUs is empty.
//
// The second goroutine must not call PinThread on the caller's thread.
// UnlockOSThread lets it run there. That call replaces the caller's CPU set.
// The wait keeps the caller locked until the second pin returns.
//
// Returns an error when either pin fails. fn does not run when a pin fails.
func (c Config) RunPinnedPair(first, second int, fn func()) error {
	return c.runPinnedPair(first, second, nil, fn)
}

// runPinnedPair is RunPinnedPair. beforeUnlock runs on each goroutine while
// it is still locked, after both pins have succeeded. nil skips it.
func (c Config) runPinnedPair(first, second int, beforeUnlock, fn func()) error {
	if len(c.CPUs) == 0 {
		go fn()
		return nil
	}
	if err := c.pinLocked(first); err != nil {
		return err
	}
	pinned := make(chan error, 1)
	release := make(chan struct{})
	go func() {
		err := c.pinLocked(second)
		if err == nil && beforeUnlock != nil {
			beforeUnlock()
		}
		pinned <- err
		if err != nil {
			return
		}
		<-release
		c.unlockHot()
		fn()
	}()
	err := <-pinned
	if err == nil && beforeUnlock != nil {
		beforeUnlock()
	}
	close(release)
	c.unlockHot()
	return err
}

// pinLocked binds the calling goroutine to CPUs[index] and leaves it locked.
func (c Config) pinLocked(index int) error {
	if index < 0 || index >= len(c.CPUs) {
		return fmt.Errorf("pin hot thread %d: -cpus lists %d cpus", index, len(c.CPUs))
	}
	return PinThread(c.CPUs[index])
}

// unlockHot unlocks the calling goroutine when UnlockThreads is set.
func (c Config) unlockHot() {
	if c.UnlockThreads {
		runtime.UnlockOSThread()
	}
}
