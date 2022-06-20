package orders

var AlreadyExistsError *alreadyExistsErr

type alreadyExistsErr struct{}

func (e *alreadyExistsErr) Error() string {
	return "number already exists"
}

func ThrowAlreadyExistsErr() *alreadyExistsErr {
	return &alreadyExistsErr{}
}

//

var BelongsToAnotherError *belongToAnotherErr

type belongToAnotherErr struct{}

func (e *belongToAnotherErr) Error() string {
	return "belong to another user"
}

func ThrowBelongToAnotherErr() *belongToAnotherErr {
	return &belongToAnotherErr{}
}

//

var InvalidNumberError *invalidNumberErr

type invalidNumberErr struct{}

func (e *invalidNumberErr) Error() string {
	return "invalid number"
}

func ThrowInvalidNumberErr() *invalidNumberErr {
	return &invalidNumberErr{}
}

//

var InvalidRequestError *invalidRequestErr

type invalidRequestErr struct{}

func (e *invalidRequestErr) Error() string {
	return "invalid request"
}

func ThrowInvalidRequestErr() *invalidRequestErr {
	return &invalidRequestErr{}
}

//

var NoItemsError *noItemsErr

type noItemsErr struct{}

func (e *noItemsErr) Error() string {
	return "no items"
}

func ThrowNoItemsErr() *noItemsErr {
	return &noItemsErr{}
}

//
