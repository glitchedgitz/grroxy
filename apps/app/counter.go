package app

import (
	"log"
	"sync"
	"sync/atomic"

	"github.com/glitchedgitz/pocketbase"
	"github.com/glitchedgitz/pocketbase/models"
)

// Counter represents a counter entry with atomic operations
type Counter struct {
	ID              string
	Key             string       // counter_key - the main identifier
	Collection      string       // optional metadata
	Filter          string       // optional metadata
	Count           atomic.Int64 // Use atomic for lock-free operations
	LoadOnStartup   bool         // if true, recalculate from DB on startup, no immediate sync
	lastSyncedValue int64        // Last value written to DB (for change detection)
}

// CounterManager manages counters in memory with database sync
type CounterManager struct {
	mu       sync.RWMutex
	counters map[string]*Counter // map[counter_key]*Counter - simple and fast!
	app      *pocketbase.PocketBase
}

// SetupCounterManager initializes the counter manager and loads from database
func (backend *Backend) SetupCounterManager() error {
	backend.CounterManager = &CounterManager{
		counters: make(map[string]*Counter),
		app:      backend.App,
	}

	log.Println("[Startup] Loading counters from database...")
	if err := backend.CounterManager.LoadFromDB(); err != nil {
		log.Printf("[Startup] Error loading counters: %v", err)
		return err
	}
	return nil
}

// LoadFromDB loads all counters from the database into memory
func (cm *CounterManager) LoadFromDB() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	records, err := cm.app.Dao().FindRecordsByExpr("_counters")
	if err != nil {
		log.Printf("[CounterManager][LoadFromDB] Error loading counters: %v\n", err)
		return err
	}

	for _, record := range records {
		countValue := int64(record.GetInt("count"))
		counter := &Counter{
			ID:              record.Id,
			Key:             record.GetString("counter_key"),
			Collection:      record.GetString("collection"),
			Filter:          record.GetString("filter"),
			LoadOnStartup:   record.GetBool("load_on_startup"),
			lastSyncedValue: countValue, // Initialize with DB value
		}
		counter.Count.Store(countValue)

		// Use counter_key as the map key - simple and fast!
		cm.counters[counter.Key] = counter
	}

	log.Printf("[CounterManager][LoadFromDB] Loaded %d counters from database\n", len(cm.counters))
	return nil
}

// Get returns the count for a given counter key (lock-free read)
// Parameters: key (counter_key - main identifier), collection (optional - updates if exists), filter (optional - updates if exists)
func (cm *CounterManager) Get(key, collection, filter string) int64 {
	cm.mu.RLock()
	counter, exists := cm.counters[key]
	cm.mu.RUnlock()

	if exists {
		return counter.Count.Load() // Atomic read, no lock needed
	}
	return 0
}

// Set sets the count for a given counter key
// Parameters: key (counter_key - main identifier), collection (optional - updates if provided), filter (optional - updates if provided), count
func (cm *CounterManager) Set(key, collection, filter string, count int64) {
	cm.mu.RLock()
	counter, exists := cm.counters[key]
	cm.mu.RUnlock()

	if exists {
		// Update metadata if provided
		if collection != "" && counter.Collection != collection {
			counter.Collection = collection
		}
		if filter != "" && counter.Filter != filter {
			counter.Filter = filter
		}
		counter.Count.Store(count) // Atomic write, no lock needed
	} else {
		// Only lock when adding new entry to map
		cm.mu.Lock()
		// Double-check after acquiring lock
		if counter, exists := cm.counters[key]; exists {
			cm.mu.Unlock()
			if collection != "" && counter.Collection != collection {
				counter.Collection = collection
			}
			if filter != "" && counter.Filter != filter {
				counter.Filter = filter
			}
			counter.Count.Store(count)
		} else {
			newCounter := &Counter{
				Key:        key,
				Collection: collection,
				Filter:     filter,
			}
			newCounter.Count.Store(count)
			cm.counters[key] = newCounter
			cm.mu.Unlock()
		}
	}
}

// IncrementWithStartup increments with explicit loadOnStartup flag
func (cm *CounterManager) IncrementWithStartup(key, collection, filter string, loadOnStartup bool) int64 {
	cm.mu.RLock()
	counter, exists := cm.counters[key]
	cm.mu.RUnlock()

	if exists {
		// Update metadata if provided and different
		if collection != "" && counter.Collection != collection {
			counter.Collection = collection
		}
		if filter != "" && counter.Filter != filter {
			counter.Filter = filter
		}
		counter.LoadOnStartup = loadOnStartup
		newCount := counter.Count.Add(1)

		// Immediate sync if not load_on_startup
		if !loadOnStartup {
			go cm.SyncOne(key)
		}
		return newCount
	}

	// Only lock when adding new entry to map
	cm.mu.Lock()
	// Double-check after acquiring lock
	if counter, exists := cm.counters[key]; exists {
		cm.mu.Unlock()
		if collection != "" && counter.Collection != collection {
			counter.Collection = collection
		}
		if filter != "" && counter.Filter != filter {
			counter.Filter = filter
		}
		counter.LoadOnStartup = loadOnStartup
		newCount := counter.Count.Add(1)

		// Immediate sync if not load_on_startup
		if !loadOnStartup {
			go cm.SyncOne(key)
		}
		return newCount
	}

	newCounter := &Counter{
		Key:           key,
		Collection:    collection,
		Filter:        filter,
		LoadOnStartup: loadOnStartup,
	}
	newCounter.Count.Store(1)
	cm.counters[key] = newCounter
	cm.mu.Unlock()

	// Immediate sync if not load_on_startup
	if !loadOnStartup {
		go cm.SyncOne(key)
	}
	return 1
}

// Increment increments the count for a given counter key (lock-free atomic operation)
// Parameters: key (counter_key - main identifier), collection (optional - updates metadata), filter (optional - updates metadata)
// Default: loadOnStartup=false (immediate sync for exact counts)
func (cm *CounterManager) Increment(key, collection, filter string) int64 {
	cm.mu.RLock()
	counter, exists := cm.counters[key]
	cm.mu.RUnlock()

	if exists {
		// Update metadata if provided and different
		if collection != "" && counter.Collection != collection {
			counter.Collection = collection
		}
		if filter != "" && counter.Filter != filter {
			counter.Filter = filter
		}
		newCount := counter.Count.Add(1) // Atomic increment, no lock needed

		// Immediate sync if not load_on_startup
		if !counter.LoadOnStartup {
			go cm.SyncOne(key)
		}
		return newCount
	}

	// Only lock when adding new entry to map
	cm.mu.Lock()
	// Double-check after acquiring lock
	if counter, exists := cm.counters[key]; exists {
		cm.mu.Unlock()
		if collection != "" && counter.Collection != collection {
			counter.Collection = collection
		}
		if filter != "" && counter.Filter != filter {
			counter.Filter = filter
		}
		newCount := counter.Count.Add(1)

		// Immediate sync if not load_on_startup
		if !counter.LoadOnStartup {
			go cm.SyncOne(key)
		}
		return newCount
	}

	newCounter := &Counter{
		Key:           key,
		Collection:    collection,
		Filter:        filter,
		LoadOnStartup: false, // Default: immediate sync
	}
	newCounter.Count.Store(1)
	cm.counters[key] = newCounter
	cm.mu.Unlock()

	// Immediate sync for new counter
	go cm.SyncOne(key)
	return 1
}

// Decrement decrements the count for a given counter key (lock-free atomic operation)
// Parameters: key (counter_key - main identifier), collection (optional - updates metadata), filter (optional - updates metadata)
// If loadOnStartup=false, syncs immediately to DB for exact counts
func (cm *CounterManager) Decrement(key, collection, filter string) int64 {
	cm.mu.RLock()
	counter, exists := cm.counters[key]
	cm.mu.RUnlock()

	if exists {
		// Update metadata if provided and different
		if collection != "" && counter.Collection != collection {
			counter.Collection = collection
		}
		if filter != "" && counter.Filter != filter {
			counter.Filter = filter
		}
		// Atomic decrement with minimum value of 0
		var newCount int64
		for {
			current := counter.Count.Load()
			if current <= 0 {
				return 0
			}
			if counter.Count.CompareAndSwap(current, current-1) {
				newCount = current - 1
				break
			}
		}

		// Immediate sync if not load_on_startup
		if !counter.LoadOnStartup {
			go cm.SyncOne(key)
		}
		return newCount
	}

	return 0
}

// SyncToDB syncs only load_on_startup counters to the database
// (load_on_startup=false counters are synced immediately via SyncOne)
func (cm *CounterManager) SyncToDB() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	collection, err := cm.app.Dao().FindCollectionByNameOrId("_counters")
	if err != nil {
		// log.Printf("[CounterManager][SyncToDB] Error finding collection: %v\n", err)
		return err
	}

	syncCount := 0
	skippedCount := 0
	for _, counter := range cm.counters {
		// Skip counters that are synced immediately
		if !counter.LoadOnStartup {
			continue
		}

		currentCount := counter.Count.Load() // Atomic read

		// Skip if value hasn't changed since last sync
		if currentCount == counter.lastSyncedValue && counter.ID != "" {
			skippedCount++
			continue
		}

		if counter.ID == "" {
			// New counter - create in DB
			record := models.NewRecord(collection)
			record.Set("counter_key", counter.Key)
			record.Set("collection", counter.Collection)
			record.Set("filter", counter.Filter)
			record.Set("count", currentCount)
			record.Set("load_on_startup", counter.LoadOnStartup)

			if err := cm.app.Dao().SaveRecord(record); err != nil {
				// log.Printf("[CounterManager][SyncToDB] Error creating counter: %v\n", err)
				continue
			}
			counter.ID = record.Id
			counter.lastSyncedValue = currentCount
			syncCount++
		} else {
			// Existing counter - update in DB
			record, err := cm.app.Dao().FindRecordById("_counters", counter.ID)
			if err != nil {
				// log.Printf("[CounterManager][SyncToDB] Error finding counter %s: %v\n", counter.ID, err)
				continue
			}

			record.Set("count", currentCount)
			if err := cm.app.Dao().SaveRecord(record); err != nil {
				// log.Printf("[CounterManager][SyncToDB] Error updating counter %s: %v\n", counter.ID, err)
				continue
			}
			counter.lastSyncedValue = currentCount
			syncCount++
		}
	}

	if skippedCount > 0 {
		// log.Printf("[CounterManager][SyncToDB] Synced %d counters, skipped %d unchanged\n", syncCount, skippedCount)
	} else {
		// log.Printf("[CounterManager][SyncToDB] Synced %d load_on_startup counters to database\n", syncCount)
	}
	return nil
}

// SyncOne syncs a single counter to database immediately (for load_on_startup=false counters)
func (cm *CounterManager) SyncOne(key string) error {
	cm.mu.RLock()
	counter, exists := cm.counters[key]
	cm.mu.RUnlock()

	if !exists {
		return nil
	}

	// Skip sync for load_on_startup counters (they're recalculated from DB)
	if counter.LoadOnStartup {
		return nil
	}

	collection, err := cm.app.Dao().FindCollectionByNameOrId("_counters")
	if err != nil {
		return err
	}

	currentCount := counter.Count.Load()

	if counter.ID == "" {
		// New counter - create in DB
		record := models.NewRecord(collection)
		record.Set("counter_key", counter.Key)
		record.Set("collection", counter.Collection)
		record.Set("filter", counter.Filter)
		record.Set("count", currentCount)
		record.Set("load_on_startup", counter.LoadOnStartup)

		if err := cm.app.Dao().SaveRecord(record); err != nil {
			return err
		}
		counter.ID = record.Id
	} else {
		// Existing counter - update in DB
		record, err := cm.app.Dao().FindRecordById("_counters", counter.ID)
		if err != nil {
			return err
		}

		record.Set("count", currentCount)
		if err := cm.app.Dao().SaveRecord(record); err != nil {
			return err
		}
	}

	return nil
}

// GetAll returns all counters (snapshot with current counts)
func (cm *CounterManager) GetAll() map[string]*Counter {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]*Counter, len(cm.counters))
	for k, v := range cm.counters {
		snapshot := &Counter{
			ID:         v.ID,
			Key:        v.Key,
			Collection: v.Collection,
			Filter:     v.Filter,
		}
		snapshot.Count.Store(v.Count.Load()) // Atomic read
		result[k] = snapshot
	}
	return result
}

// Reset resets a specific counter to 0 (lock-free atomic operation)
// Parameters: key (counter_key - main identifier), collection (optional - updates metadata), filter (optional - updates metadata)
func (cm *CounterManager) Reset(key, collection, filter string) {
	cm.mu.RLock()
	counter, exists := cm.counters[key]
	cm.mu.RUnlock()

	if exists {
		// Update metadata if provided and different
		if collection != "" && counter.Collection != collection {
			counter.Collection = collection
		}
		if filter != "" && counter.Filter != filter {
			counter.Filter = filter
		}
		counter.Count.Store(0) // Atomic write, no lock needed
	}
}

// Delete removes a counter from memory and database
// Parameters: key (counter_key - main identifier), collection (not used), filter (not used)
func (cm *CounterManager) Delete(key, collection, filter string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if counter, exists := cm.counters[key]; exists {
		// Delete from DB if it exists there
		if counter.ID != "" {
			record, err := cm.app.Dao().FindRecordById("_counters", counter.ID)
			if err == nil {
				if err := cm.app.Dao().DeleteRecord(record); err != nil {
					log.Printf("[CounterManager][Delete] Error deleting counter from DB: %v\n", err)
				}
			}
		}
		delete(cm.counters, key)
	}
}

// GetDetails returns the full counter details (key, collection, filter, count)
// This makes it easy to find all the details: counters["my_key"] gives you everything!
func (cm *CounterManager) GetDetails(key string) (*Counter, bool) {
	cm.mu.RLock()
	counter, exists := cm.counters[key]
	cm.mu.RUnlock()

	if exists {
		// Return a copy with current count
		snapshot := &Counter{
			ID:         counter.ID,
			Key:        counter.Key,
			Collection: counter.Collection,
			Filter:     counter.Filter,
		}
		snapshot.Count.Store(counter.Count.Load())
		return snapshot, true
	}
	return nil, false
}
