package birthdays

import (
	"testing"
	"time"
)

func TestBirthdaysByNameInAnyCase(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Set("Anna", "2015-05-12"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("anna", "2015-05-13"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("Ben", "2999-01-01"); err != ErrBadDate {
		t.Errorf("a birthday to come: %v, want ErrBadDate", err)
	}
	all, err := s.All()
	if err != nil || len(all) != 1 || all["anna"] != "2015-05-13" {
		t.Fatalf("birthdays %v, %v", all, err)
	}
	birth, ok := Of(all, "ANNA")
	if !ok {
		t.Fatal("Anna's birthday not found")
	}
	if age := AgeAt(birth, time.Date(2020, 5, 12, 0, 0, 0, 0, time.UTC)); age != 4 {
		t.Errorf("age the day before the birthday: %d, want 4", age)
	}
	if age := AgeAt(birth, time.Date(2020, 5, 13, 0, 0, 0, 0, time.UTC)); age != 5 {
		t.Errorf("age on the birthday: %d, want 5", age)
	}
	if err := s.Set("Anna", ""); err != nil {
		t.Fatal(err)
	}
	if all, _ := s.All(); len(all) != 0 {
		t.Errorf("after taking it away: %v", all)
	}
}
