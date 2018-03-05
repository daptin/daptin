package auth

import (
	"fmt"
	"testing"
)

func TestAllPermission(t *testing.T) {

	perm1 := NewPermission(Read, Read|Create|Update, CRUD)
	perm2 := NewPermission(Create|Read|Refer, Read|Update|Execute, Create|Read|Refer|Update)
	perm3 := NewPermission(None, Read|Execute, CRUD|Execute)
	perm4 := NewPermission(Read, Read|Execute, CRUD|Execute)
	perm5 := NewPermission(Peek|ExecuteStrict, Read|Execute, CRUD|Execute)

	tperm1 := ParsePermission(perm1.IntValue())
	tperm2 := ParsePermission(perm2.IntValue())
	//tperm2 := ParsePermission(perm3.IntValue())

	if perm1 == perm2 {
		t.Errorf("Permission should not be equal")
	}

	if perm1 != tperm1 {
		t.Errorf("Parsing failed")
	}

	if perm2 != tperm2 {
		t.Errorf("Parsing failed")
	}
	fmt.Printf("Perm 1: %v == %v == %v\n", perm1, perm1.IntValue(), tperm1.IntValue())
	fmt.Printf("Perm 2: %v == %v == %v\n", perm2, perm2.IntValue(), tperm2.IntValue())
	fmt.Printf("Perm 3: %v == %v\n", perm3, perm3.IntValue())
	fmt.Printf("Perm 4: %v == %v\n", perm4, perm4.IntValue())
	fmt.Printf("Perm 5: %v == %v\n", perm5, perm5.IntValue())

}

func TestAuthPermissions(t *testing.T) {

	t.Logf("Permission [%v] %v", None, None)

	t.Logf("Permission [%v] %v", Peek, int64(Peek))
	t.Logf("Permission [%v] %v", ReadStrict, int64(ReadStrict))
	t.Logf("Permission [%v] %v", CreateStrict, int64(CreateStrict))
	t.Logf("Permission [%v] %v", UpdateStrict, int64(UpdateStrict))
	t.Logf("Permission [%v] %v", DeleteStrict, int64(DeleteStrict))
	t.Logf("Permission [%v] %v", ExecuteStrict, int64(ExecuteStrict))
	t.Logf("Permission [%v] %v", ReferStrict, int64(ReferStrict))
	t.Logf("Permission [%v] %v", Read, int64(Read))
	t.Logf("Permission [%v] %v", Refer, int64(Refer))
	t.Logf("Permission [%v] %v", Create, int64(Create))
	t.Logf("Permission [%v] %v", Update, int64(Update))
	t.Logf("Permission [%v] %v", Delete, int64(Delete))
	t.Logf("Permission [%v] %v", Execute, int64(Execute))
	t.Logf("Permission [%v] %v", CRUD, int64(CRUD))

	AllPermissions := []AuthPermission{None, Peek, ReadStrict, CreateStrict, UpdateStrict, DeleteStrict, ExecuteStrict, ReferStrict, Read, Refer, Create, Update, Delete, Execute, CRUD}

	for i, p1 := range AllPermissions {
		for j, p2 := range AllPermissions {
			if i == j {
				continue
			}

			if p1 == p2 {
				t.Errorf("Permissions are equal [%v] == [%v]", p1, p2)
			}

		}
	}

}
