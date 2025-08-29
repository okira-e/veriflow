package opt

import "encoding/json"

// Option is a type that represents an optional value.
type Option[T any] struct {
	val    T
	exists bool
}

// Some creates a new Option[T] with the given value.
func Some[T any](val T) Option[T] {
	return Option[T]{
		val:    val,
		exists: true,
	}
}

// None creates a new empty Option[T].
func None[T any]() Option[T] {
	return Option[T]{
		exists: false,
	}
}

// IsSome returns true if the option is a Some value.
func (o *Option[T]) IsSome() bool {
	return o.exists
}

// IsNone returns true if the option is a None value.
func (o *Option[T]) IsNone() bool {
	return !o.exists
}

// Expect panics with the given message if the option is a None value.
func (o *Option[T]) Expect(msg string) T {
	if o.exists {
		return o.val
	}

	panic(msg)
}

// Unwrap panics if the option is a None value.
func (o *Option[T]) Unwrap() T {
	if o.exists {
		return o.val
	}

	panic("unwrap a none option")
}

// UnwrapOr returns the value of the option if it is a Some value, otherwise it returns the given default value.
func (o *Option[T]) UnwrapOr(def T) T {
	if o.exists {
		return o.val
	}

	return def
}

// UnwrapOrElse returns the value of the option if it is a Some value, otherwise it returns the result of the given function.
func (o *Option[T]) UnwrapOrElse(fn func() T) T {
	if o.exists {
		return o.val
	}

	return fn()
}

// --- json support ---

// MarshalJSON emits the inner value when Some, or null when None.
func (o Option[T]) MarshalJSON() ([]byte, error) {
	if !o.exists {
		return []byte("null"), nil
	}
	return json.Marshal(o.val)
}

// UnmarshalJSON sets None on "null", or decodes T and sets Some.
func (o *Option[T]) UnmarshalJSON(b []byte) error {
	// Missing fields won't call this; the zero value remains None.
	if string(b) == "null" {
		*o = None[T]()
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*o = Some(v)
	return nil
}

// IsZero lets `omitempty` drop this field when it's None (Go 1.20+).
func (o Option[T]) IsZero() bool {
	return !o.exists
}
