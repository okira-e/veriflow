package oops

import . "github.com/okira-e/veriflow/app/utils"

type Code int

const (
	OK Code = iota
	Internal
	FlowNotFound
	StepNotFound
	EmptyFlows
	StepAlreadyExists
	PromptError
	UserAborted
	OperationFailed
	ConfigFileNotFound
	ConfigFileExistsError
	ErrInvalidInput
	MissingRequiredFlag
	AssertionExpressionParsingFailure
	InvalidTarget
	// File operation errors
	FileReadError
	FileWriteError
	FileNotFound
	// Flow operation errors
	FlowExecutionFailed
	FlowAlreadyExists
	FlowRemovalError
	FlowUpdateError
	// Step operation errors
	StepUnmarshalError
	StepMarshalError
	StepExecutionFailed
	StepRequestFailed
	StepRequestBuildFailed
	StepRequestProcessingFailed
	StepResponseReadFailed
	StepRequestDeadlineExceeded
	StepRequestAssertionFailed
	StepRequestReturnedNotFound
	StepResponseEmpty
	StepRequestStatusMismatch
	StepRequestResponseAssertionFailed
	StepRequestResponseParsingFailure
	StepRequestResponseKeyNotFound
	StepRequestResponseKeyForbidden
	StepRequestResponseValueMismatch
	StepExportFailed
	BeforeRunFailed
	AfterRunFailed
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
	// JSONPath validation errors
	JSONPathValidationError
)

func (code Code) String() string {
	switch code {
	case OK:
		return PascalToScreamingSnake("OK")
	case Internal:
		return PascalToScreamingSnake("Internal")
	case FlowNotFound:
		return PascalToScreamingSnake("FlowNotFound")
	case StepNotFound:
		return PascalToScreamingSnake("StepNotFound")
	case EmptyFlows:
		return PascalToScreamingSnake("EmptyFlows")
	case StepAlreadyExists:
		return PascalToScreamingSnake("StepAlreadyExists")
	case PromptError:
		return PascalToScreamingSnake("PromptError")
	case UserAborted:
		return PascalToScreamingSnake("UserAborted")
	case OperationFailed:
		return PascalToScreamingSnake("OperationFailed")
	case ConfigFileNotFound:
		return PascalToScreamingSnake("ConfigFileNotFound")
	case ConfigFileExistsError:
		return PascalToScreamingSnake("ConfigFileExistsError")
	case ErrInvalidInput:
		return PascalToScreamingSnake("ErrInvalidInput")
	case MissingRequiredFlag:
		return PascalToScreamingSnake("MissingRequiredFlag")
	case AssertionExpressionParsingFailure:
		return PascalToScreamingSnake("AssertionExpressionParsingFailure")
	case InvalidTarget:
		return PascalToScreamingSnake("InvalidTarget")
	case FileReadError:
		return PascalToScreamingSnake("FileReadError")
	case FileWriteError:
		return PascalToScreamingSnake("FileWriteError")
	case FileNotFound:
		return PascalToScreamingSnake("FileNotFound")
	case FlowExecutionFailed:
		return PascalToScreamingSnake("FlowExecutionFailed")
	case FlowAlreadyExists:
		return PascalToScreamingSnake("FlowAlreadyExists")
	case FlowRemovalError:
		return PascalToScreamingSnake("FlowRemovalError")
	case FlowUpdateError:
		return PascalToScreamingSnake("FlowUpdateError")
	case StepExecutionFailed:
		return PascalToScreamingSnake("StepExecutionFailed")
	case StepUnmarshalError:
		return PascalToScreamingSnake("StepUnmarshalError")
	case StepMarshalError:
		return PascalToScreamingSnake("StepMarshalError")
	case StepRequestFailed:
		return PascalToScreamingSnake("StepRequestFailed")
	case StepRequestReturnedNotFound:
		return PascalToScreamingSnake("StepRequestReturnedNotFound")
	case StepRequestBuildFailed:
		return PascalToScreamingSnake("StepRequestBuildFailed")
	case StepRequestProcessingFailed:
		return PascalToScreamingSnake("StepRequestProcessingFailed")
	case StepResponseReadFailed:
		return PascalToScreamingSnake("StepResponseReadFailed")
	case StepRequestStatusMismatch:
		return PascalToScreamingSnake("StepRequestStatusMismatch")
	case StepRequestResponseAssertionFailed:
		return PascalToScreamingSnake("StepRequestResponseAssertionFailed")
	case StepRequestResponseParsingFailure:
		return PascalToScreamingSnake("StepRequestResponseParsingFailure")
	case StepRequestResponseKeyNotFound:
		return PascalToScreamingSnake("StepRequestResponseKeyNotFound")
	case StepRequestResponseKeyForbidden:
		return PascalToScreamingSnake("StepRequestResponseKeyForbidden")
	case StepRequestResponseValueMismatch:
		return PascalToScreamingSnake("StepRequestResponseValueMismatch")
	case StepExportFailed:
		return PascalToScreamingSnake("StepExportFailed")
	case BeforeRunFailed:
		return PascalToScreamingSnake("BeforeRunFailed")
	case AfterRunFailed:
		return PascalToScreamingSnake("AfterRunFailed")
	case StepResponseEmpty:
		return PascalToScreamingSnake("StepResponseEmpty")
	case StepRequestDeadlineExceeded:
		return PascalToScreamingSnake("StepRequestDeadlineExceeded")
	case StepRequestAssertionFailed:
		return PascalToScreamingSnake("StepRequestAssertionFailed")
	case ConfigCreationError:
		return PascalToScreamingSnake("ConfigCreationError")
	case ConfigMarshalError:
		return PascalToScreamingSnake("ConfigMarshalError")
	case ConfigUnmarshalError:
		return PascalToScreamingSnake("ConfigUnmarshalError")
	case NetworkError:
		return PascalToScreamingSnake("NetworkError")
	case ConnectionFailed:
		return PascalToScreamingSnake("ConnectionFailed")
	case TimeoutError:
		return PascalToScreamingSnake("TimeoutError")
	case ValidationError:
		return PascalToScreamingSnake("ValidationError")
	case HTTPError:
		return PascalToScreamingSnake("HTTPError")
	case JSONParseError:
		return PascalToScreamingSnake("JSONParseError")
	case AuthenticationError:
		return PascalToScreamingSnake("AuthenticationError")
	case JSONPathValidationError:
		return PascalToScreamingSnake("JSONPathValidationError")
	default:
		return "UNKNOWN"
	}
}

func (code Code) IsUserError() bool {
	switch code {
	case FlowNotFound, StepNotFound, EmptyFlows, StepAlreadyExists, UserAborted, ErrInvalidInput,
		ValidationError, FlowAlreadyExists, ConfigFileNotFound, ConfigFileExistsError, ConfigMarshalError, ConfigUnmarshalError, JSONParseError, FileNotFound, FlowRemovalError, StepRequestFailed, StepRequestBuildFailed, StepRequestProcessingFailed, StepUnmarshalError, StepMarshalError, StepResponseReadFailed, StepResponseEmpty, BeforeRunFailed, AfterRunFailed, MissingRequiredFlag, InvalidTarget, AssertionExpressionParsingFailure, JSONPathValidationError:
		return true
	}

	return false
}
