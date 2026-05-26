package core

type Storage interface {
	WriteUser(user User) error
	ReadUser(id string) (User, error)
	SaveAssignment(assignment Assignment) error
	GetAssignment(assignmentId string) (Assignment, error)
	ListAssignments() ([]Assignment, error)
}
