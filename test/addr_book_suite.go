package test

import (
	"fmt"
	"testing"
	"time"

	peer "github.com/libp2p/go-libp2p-peer"
	pt "github.com/libp2p/go-libp2p-peer/test"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

var addressBookSuite = map[string]func(book pstore.AddrBook) func(*testing.T){
	"AddAddress":           testAddAddress,
	"Clear":                testClearWorks,
	"SetNegativeTTLClears": testSetNegativeTTLClears,
	"UpdateTTLs":           testUpdateTTLs,
	"NilAddrsDontBreak":    testNilAddrsDontBreak,
	"AddressesExpire":      testAddressesExpire,
	"ClearWithIter":        testClearWithIterator,
	"PeersWithAddresses":   testPeersWithAddrs,
}

type AddrBookFactory func() (pstore.AddrBook, func())

func TestAddrBook(t *testing.T, factory AddrBookFactory) {
	for name, test := range addressBookSuite {
		// Create a new peerstore.
		ab, closeFunc := factory()

		// Run the test.
		t.Run(name, test(ab))

		// Cleanup.
		if closeFunc != nil {
			closeFunc()
		}
	}
}

func generateAddrs(count int) []ma.Multiaddr {
	var addrs = make([]ma.Multiaddr, count)
	for i := 0; i < count; i++ {
		addrs[i] = multiaddr(fmt.Sprintf("/ip4/1.1.1.%d/tcp/1111", i))
	}
	return addrs
}

func generatePeerIds(count int) []peer.ID {
	var ids = make([]peer.ID, count)
	for i := 0; i < count; i++ {
		ids[i], _ = pt.RandPeerID()
	}
	return ids
}

func testAddAddress(ab pstore.AddrBook) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("add a single address", func(t *testing.T) {
			id := generatePeerIds(1)[0]
			addrs := generateAddrs(1)

			ab.AddAddr(id, addrs[0], time.Hour)

			testHas(t, addrs, ab.Addrs(id))
		})

		t.Run("idempotent add single address", func(t *testing.T) {
			id := generatePeerIds(1)[0]
			addrs := generateAddrs(1)

			ab.AddAddr(id, addrs[0], time.Hour)
			ab.AddAddr(id, addrs[0], time.Hour)

			testHas(t, addrs, ab.Addrs(id))
		})

		t.Run("add multiple addresses", func(t *testing.T) {
			id := generatePeerIds(1)[0]
			addrs := generateAddrs(3)

			ab.AddAddrs(id, addrs, time.Hour)
			testHas(t, addrs, ab.Addrs(id))
		})

		t.Run("idempotent add multiple addresses", func(t *testing.T) {
			id := generatePeerIds(1)[0]
			addrs := generateAddrs(3)

			ab.AddAddrs(id, addrs, time.Hour)
			ab.AddAddrs(id, addrs, time.Hour)

			testHas(t, addrs, ab.Addrs(id))
		})
	}
}

func testClearWorks(ab pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		ids := generatePeerIds(2)
		addrs := generateAddrs(5)

		ab.AddAddrs(ids[0], addrs[0:3], time.Hour)
		ab.AddAddrs(ids[1], addrs[3:], time.Hour)

		testHas(t, addrs[0:3], ab.Addrs(ids[0]))
		testHas(t, addrs[3:], ab.Addrs(ids[1]))

		ab.ClearAddrs(ids[0])
		testHas(t, nil, ab.Addrs(ids[0]))
		testHas(t, addrs[3:], ab.Addrs(ids[1]))

		ab.ClearAddrs(ids[1])
		testHas(t, nil, ab.Addrs(ids[0]))
		testHas(t, nil, ab.Addrs(ids[1]))
	}
}

func testSetNegativeTTLClears(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		id := generatePeerIds(1)[0]
		addr := generateAddrs(1)[0]

		m.SetAddr(id, addr, time.Hour)
		testHas(t, []ma.Multiaddr{addr}, m.Addrs(id))

		m.SetAddr(id, addr, -1)
		testHas(t, nil, m.Addrs(id))
	}
}

func testUpdateTTLs(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		t.Run("update ttl of peer with no addrs", func(t *testing.T) {
			id := generatePeerIds(1)[0]

			// Shouldn't panic.
			m.UpdateAddrs(id, time.Hour, time.Minute)
		})

		t.Run("update ttls successfully", func(t *testing.T) {
			ids := generatePeerIds(2)
			addrs1, addrs2 := generateAddrs(2), generateAddrs(2)

			// set two keys with different ttls for each peer.
			m.SetAddr(ids[0], addrs1[0], time.Hour)
			m.SetAddr(ids[0], addrs1[1], time.Minute)
			m.SetAddr(ids[1], addrs2[0], time.Hour)
			m.SetAddr(ids[1], addrs2[1], time.Minute)

			// Sanity check.
			testHas(t, addrs1, m.Addrs(ids[0]))
			testHas(t, addrs2, m.Addrs(ids[1]))

			// Will only affect addrs1[0].
			m.UpdateAddrs(ids[0], time.Hour, time.Second)

			// No immediate effect.
			testHas(t, addrs1, m.Addrs(ids[0]))
			testHas(t, addrs2, m.Addrs(ids[1]))

			// After a wait, addrs[0] is gone.
			time.Sleep(1200 * time.Millisecond)
			testHas(t, addrs1[1:2], m.Addrs(ids[0]))
			testHas(t, addrs2, m.Addrs(ids[1]))

			// Will only affect addrs2[0].
			m.UpdateAddrs(ids[1], time.Hour, time.Second)

			// No immediate effect.
			testHas(t, addrs1[1:2], m.Addrs(ids[0]))
			testHas(t, addrs2, m.Addrs(ids[1]))

			time.Sleep(1200 * time.Millisecond)

			// First addrs is gone in both.
			testHas(t, addrs1[1:], m.Addrs(ids[0]))
			testHas(t, addrs2[1:], m.Addrs(ids[1]))
		})

	}
}

func testNilAddrsDontBreak(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		id := generatePeerIds(1)[0]

		m.SetAddr(id, nil, time.Hour)
		m.AddAddr(id, nil, time.Hour)
	}
}

func testAddressesExpire(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		ids := generatePeerIds(2)
		addrs1 := generateAddrs(3)
		addrs2 := generateAddrs(2)

		m.AddAddrs(ids[0], addrs1, time.Hour)
		m.AddAddrs(ids[1], addrs2, time.Hour)

		testHas(t, addrs1, m.Addrs(ids[0]))
		testHas(t, addrs2, m.Addrs(ids[1]))

		m.AddAddrs(ids[0], addrs1, 2*time.Hour)
		m.AddAddrs(ids[1], addrs2, 2*time.Hour)

		testHas(t, addrs1, m.Addrs(ids[0]))
		testHas(t, addrs2, m.Addrs(ids[1]))

		m.SetAddr(ids[0], addrs1[0], time.Millisecond)
		<-time.After(time.Millisecond * 5)
		testHas(t, addrs1[1:3], m.Addrs(ids[0]))
		testHas(t, addrs2, m.Addrs(ids[1]))

		m.SetAddr(ids[0], addrs1[2], time.Millisecond)
		<-time.After(time.Millisecond * 5)
		testHas(t, addrs1[1:2], m.Addrs(ids[0]))
		testHas(t, addrs2, m.Addrs(ids[1]))

		m.SetAddr(ids[1], addrs2[0], time.Millisecond)
		<-time.After(time.Millisecond * 5)
		testHas(t, addrs1[1:2], m.Addrs(ids[0]))
		testHas(t, addrs2[1:], m.Addrs(ids[1]))

		m.SetAddr(ids[1], addrs2[1], time.Millisecond)
		<-time.After(time.Millisecond * 5)
		testHas(t, addrs1[1:2], m.Addrs(ids[0]))
		testHas(t, nil, m.Addrs(ids[1]))

		m.SetAddr(ids[0], addrs1[1], time.Millisecond)
		<-time.After(time.Millisecond * 5)
		testHas(t, nil, m.Addrs(ids[0]))
		testHas(t, nil, m.Addrs(ids[1]))
	}
}

func testClearWithIterator(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		ids := generatePeerIds(2)
		addrs := generateAddrs(100)

		// Add the peers with 50 addresses each.
		m.AddAddrs(ids[0], addrs[:50], pstore.PermanentAddrTTL)
		m.AddAddrs(ids[1], addrs[50:], pstore.PermanentAddrTTL)

		if all := append(m.Addrs(ids[0]), m.Addrs(ids[1])...); len(all) != 100 {
			t.Fatal("expected pstore to contain both peers with all their maddrs")
		}

		// Since we don't fetch these peers, they won't be present in cache.

		m.ClearAddrs(ids[0])
		if all := append(m.Addrs(ids[0]), m.Addrs(ids[1])...); len(all) != 50 {
			t.Fatal("expected pstore to contain only addrs of peer 2")
		}

		m.ClearAddrs(ids[1])
		if all := append(m.Addrs(ids[0]), m.Addrs(ids[1])...); len(all) != 0 {
			t.Fatal("expected pstore to contain no addresses")
		}
	}
}

func testPeersWithAddrs(m pstore.AddrBook) func(t *testing.T) {
	return func(t *testing.T) {
		// cannot run in parallel as the store is modified.
		// go runs sequentially in the specified order
		// see https://blog.golang.org/subtests

		t.Run("empty addrbook", func(t *testing.T) {
			if peers := m.PeersWithAddrs(); len(peers) != 0 {
				t.Fatal("expected to find no peers")
			}
		})

		t.Run("non-empty addrbook", func(t *testing.T) {
			ids := generatePeerIds(2)
			addrs := generateAddrs(10)

			m.AddAddrs(ids[0], addrs[:5], pstore.PermanentAddrTTL)
			m.AddAddrs(ids[1], addrs[5:], pstore.PermanentAddrTTL)

			if peers := m.PeersWithAddrs(); len(peers) != 2 {
				t.Fatal("expected to find 2 peers")
			}
		})
	}
}

func testHas(t *testing.T, exp, act []ma.Multiaddr) {
	t.Helper()
	if len(exp) != len(act) {
		t.Fatalf("lengths not the same. expected %d, got %d\n", len(exp), len(act))
	}

	for _, a := range exp {
		found := false

		for _, b := range act {
			if a.Equal(b) {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf("expected address %s not found", a)
		}
	}
}
