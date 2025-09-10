package oops

type Code int

const (
	OK Code = iota
	Internal
	FlowDoesntExist
	EmptyFlows
	StepAlreadyExists
	PromptError
	UserAborted
	OperationFailed
	ConfigFileNotFound
	ErrInvalidInput
	MissingRequiredFlag
	// File operation errors
	FileReadError
	FileWriteError
	FileNotFound
	// Flow operation errors
	FlowAlreadyExists
	FlowRemovalError
	FlowUpdateError
	// Config errors
	ConfigCreationError
	ConfigMarshalError
	ConfigUnmarshalError
	// Network errors
	NetworkError
	ConnectionFailed
	TimeoutError
	// Validation errors
	ValidationError
	// HTTP errors
	HTTPError
	// JSON errors
	JSONParseError
	// Authentication errors
	AuthenticationError
)

func (code Code) String() string {
	switch code {
	case OK:
		return "OK"
	case Internal:
		return "INTERNAL"
	case FlowDoesntExist:
		return "FLOW_DOESNT_EXIST"
	case EmptyFlows:
		return "EMPTY_FLOWS"
	case StepAlreadyExists:
		return "STEP_ALREADY_EXISTS"
	case PromptError:
		return "PROMPT_ERROR"
	case UserAborted:
		return "USER_ABORTED"
	case OperationFailed:
		return "OPERATION_FAILED"
	case ConfigFileNotFound:
		return "CONFIG_FILE_NOT_FOUND"
	case ErrInvalidInput:
		return "ERR_INVALID_INPUT"
	case MissingRequiredFlag:
		return "MISSING_REQUIRED_FLAG"
	case FileReadError:
		return "FILE_READ_ERROR"
	case FileWriteError:
		return "FILE_WRITE_ERROR"
	case FileNotFound:
		return "FILE_NOT_FOUND"
	case FlowAlreadyExists:
		return "FLOW_ALREADY_EXISTS"
	case FlowRemovalError:
		return "FLOW_REMOVAL_ERROR"
	case FlowUpdateError:
		return "FLOW_UPDATE_ERROR"
	case ConfigCreationError:
		return "CONFIG_CREATION_ERROR"
	case ConfigMarshalError:
		return "CONFIG_MARSHAL_ERROR"
	case ConfigUnmarshalError:
		return "CONFIG_UNMARSHAL_ERROR"
	case NetworkError:
		return "NETWORK_ERROR"
	case ConnectionFailed:
		return "CONNECTION_FAILED"
	case TimeoutError:
		return "TIMEOUT_ERROR"
	case ValidationError:
		return "VALIDATION_ERROR"
	case HTTPError:
		return "HTTP_ERROR"
	case JSONParseError:
		return "JSON_PARSE_ERROR"
	case AuthenticationError:
		return "AUTHENTICATION_ERROR"
	default:
		return "UNKNOWN"
	}
}

func (code Code) IsUserError() bool {
	switch code {
	case FlowDoesntExist, EmptyFlows, StepAlreadyExists, UserAborted, ErrInvalidInput,
		ValidationError, FlowAlreadyExists, ConfigFileNotFound, JSONParseError,
		FileNotFound, FlowRemovalError, MissingRequiredFlag:
		return true
	}

	return false
}
