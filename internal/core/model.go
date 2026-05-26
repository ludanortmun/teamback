package core

type Role int

const (
	RoleStudent Role = iota
	RoleTeacher
)

type User struct {
	ID   string
	Name string
	Role Role
}

// Assignment represents an instance of a Team's assignment.
// Each assignment has a title and description, and maps to a single feedback form.
type Assignment struct {
	ID       string
	Team     []User
	Title    string
	Feedback []Answer
}

// Answer represents a student's feedback and self-reflection.
// To be valid, it must include the student's own contribution description and weight,
// as well as descriptions and weights for each teammate's contribution. Weights must sum 100%.
type Answer struct {
	Author              User
	MemberContributions map[string]Contribution
}

type Contribution struct {
	Description string
	Weight      uint8
}
