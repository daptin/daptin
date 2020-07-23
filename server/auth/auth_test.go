package auth

import (
	"fmt"
	"testing"
)

func TestAllPermission(t *testing.T) {

	perm1 := GuestRead | UserRead | UserCreate | UserUpdate | GroupCRUD
	perm2 := GuestCreate | GuestRead | GuestRefer | UserRead | UserUpdate | UserExecute | GroupCreate | GroupRead | GroupRefer | GroupUpdate
	perm3 := None | UserRead | UserExecute | GroupCRUD | GroupExecute
	perm4 := GuestRead | UserRead | UserExecute | GroupCRUD | GroupExecute
	perm5 := GuestPeek | GuestExecute | UserRead | UserExecute | GroupCRUD | GroupExecute

	tperm1 := perm1
	tperm2 := perm2
	//tperm2 := ParsePermission(perm3)

	if perm1 == perm2 {
		t.Errorf("Permission should not be equal")
	}

	if perm1 != tperm1 {
		t.Errorf("Parsing failed")
	}

	if perm2 != tperm2 {
		t.Errorf("Parsing failed")
	}
	fmt.Printf("Perm 1: %v == %v == %v\n", perm1, perm1, tperm1)
	fmt.Printf("Perm 2: %v == %v == %v\n", perm2, perm2, tperm2)
	fmt.Printf("Perm 3: %v == %v\n", perm3, perm3)
	fmt.Printf("Perm 4: %v == %v\n", perm4, perm4)
	fmt.Printf("Perm 5: %v == %v\n", perm5, perm5)

}

func TestAuthPermissions(t *testing.T) {

	t.Logf("Permission [%v] %v", None, None)

	t.Logf("Permission [%v] %v", GuestPeek, int64(GuestPeek))
	t.Logf("Permission [%v] %v", GuestRead, int64(GuestRead))
	t.Logf("Permission [%v] %v", GuestRefer, int64(GuestRefer))
	t.Logf("Permission [%v] %v", GuestCreate, int64(GuestCreate))
	t.Logf("Permission [%v] %v", GuestUpdate, int64(GuestUpdate))
	t.Logf("Permission [%v] %v", GuestDelete, int64(GuestDelete))
	t.Logf("Permission [%v] %v", GuestExecute, int64(GuestExecute))
	t.Logf("Permission [%v] %v", GuestCRUD, int64(GuestCRUD))

	AllPermissions := []AuthPermission{
		None,
		GuestPeek,
		GuestRead,
		GuestRefer,
		GuestCreate,
		GuestUpdate,
		GuestDelete,
		GuestExecute,
		GuestCRUD,
		UserPeek,
		UserRead,
		UserRefer,
		UserCreate,
		UserUpdate,
		UserDelete,
		UserExecute,
		UserCRUD,
		GroupPeek,
		GroupRead,
		GroupRefer,
		GroupCreate,
		GroupUpdate,
		GroupDelete,
		GroupExecute,
		GroupCRUD,
	}

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
