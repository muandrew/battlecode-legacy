package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/muandrew/battlecode-ladder-go/models"
)

const (
	AddSet   = "SET"
	addLpush = "LPUSH"
)

type RdsDb struct {
	pool *redis.Pool
}

func NewRdsDb(addr string) (*RdsDb, error) {
	rdb := &RdsDb{pool: &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", addr) },
	}}
	err := rdb.Ping()
	if err != nil {
		return nil, err
	} else {
		return rdb, nil
	}
}

func (db *RdsDb) Ping() error {
	c := db.pool.Get()
	defer c.Close()
	response, err := c.Do("PING")
	if err != nil {
		return err
	} else {
		fmt.Printf("redis ping: %s\n", response)
		return nil
	}
}

func (db *RdsDb) GetUserWithApp(app string, appUuid string, generateUser func() *models.User) *models.User {
	c := db.pool.Get()
	defer c.Close()
	appKey := "oauth" + ":" + app + ":" + appUuid
	userUuid, _ := redis.String(c.Do("GET", appKey))
	if userUuid != "" {
		userBin, err := c.Do("GET", "user:"+userUuid)
		if err != nil {
			return nil
		}
		if userBin != nil {
			bin := userBin.([]byte)
			usr := &models.User{}
			json.Unmarshal([]byte(bin), usr)
			return usr
		}
	} else {
		user := generateUser()
		key := "user:" + user.Uuid
		userBin, _ := json.Marshal(user)
		c.Do("SET", key, userBin)
		c.Do("SET", appKey, user.Uuid)
		return user
	}
	return nil
}

func (db *RdsDb) GetUser(uuid string) *models.User {
	model := &models.User{}
	db.getModelForKey(model, "user:"+uuid)
	return model
}

func (db *RdsDb) GetBot(uuid string) *models.Bot {
	model := &models.Bot{}
	err := db.getModelForKey(model, getBotKeyWithUuid(uuid))
	if err != nil {
		return nil
	}
	return model
}

func (db *RdsDb) CreateBot(model *models.Bot) error {
	c := db.pool.Get()
	defer c.Close()

	err := SendModel(c, AddSet, getBotKey(model), model)
	if err != nil {
		return err
	}
	err = c.Send(addLpush, getPrefix(model.Owner)+":bot-list", model.Uuid)
	if err != nil {
		return err
	}
	_, err = flushAndReceive(c)
	return err
}

func (db *RdsDb) UpdateBot(model *models.Bot) error {
	return db.setModelForKey(model, getBotKey(model))
}

func (db *RdsDb) GetBots(userUuid string, page int, pageSize int) ([]*models.Bot, int) {
	c := db.pool.Get()
	defer c.Close()
	length, _ := redis.Int(c.Do("LLEN", "user:"+userUuid+":bot-list"))
	start := page * pageSize
	end := start + pageSize - 1
	botUuids, err := redis.Strings(c.Do("LRANGE", "user:"+userUuid+":bot-list", start, end))
	if err != nil {
		return nil, 0
	}
	bots := make([]*models.Bot, len(botUuids))

	for i, botUuid := range botUuids {
		bot := &models.Bot{}
		err = GetModel(c, getBotKeyWithUuid(botUuid), bot)
		if err != nil {
			return nil, 0
		}
		bots[i] = bot
	}
	return bots, length
}

func (db *RdsDb) GetPublicBots(page int, pageSize int) ([]*models.Bot, int) {
	c := db.pool.Get()
	defer c.Close()

	botUuids, err := redis.Strings(c.Do("ZREVRANGE", "public:bot-list", 0, -1))
	if err != nil {
		return nil, 0
	}

	length := len(botUuids)
	bots := make([]*models.Bot, length)
	for i, botUuid := range botUuids {
		bot := &models.Bot{}
		err = GetModel(c, getBotKeyWithUuid(botUuid), bot)
		if err != nil {
			return nil, 0
		}
		bots[i] = bot
	}
	return bots, length
}

func (db *RdsDb) SetPublicBot(userUuid string, botUuid string) (*models.Bot, error) {
	c := db.pool.Get()
	defer c.Close()

	currentBotUuid, _ := redis.String(c.Do("GET", "user:"+userUuid+":public-bot"))

	bot := &models.Bot{}
	err := GetModel(c, getBotKeyWithUuid(botUuid), bot)
	if err != nil {
		return nil, err
	}

	if bot.Owner.Uuid != userUuid {
		return nil, errors.New("you can only set your own bot")
	}
	if bot.Status.Status != models.BuildStatusSuccess {
		return nil, errors.New("you should only set successful bots")
	}

	// user removing public bot
	if bot == nil {
		if currentBotUuid != "" {
			err := c.Send("DEL", "user:"+userUuid+":public-bot")
			if err != nil {
				return nil, err
			}
			err = c.Send("ZREM", "public:bot-list", currentBotUuid)
			if err != nil {
				return nil, err
			}
		}
	} else {
		err = c.Send("SET", "user:"+userUuid+":public-bot", bot.Uuid)
		if err != nil {
			return nil, err
		}
		if currentBotUuid != "" {
			err = c.Send("ZREM", "public:bot-list", currentBotUuid)
			if err != nil {
				return nil, err
			}
		}
		err = c.Send("ZADD", "public:bot-list", time.Now().Unix(), bot.Uuid)
		if err != nil {
			return nil, err
		}
	}
	err = c.Flush()
	if err != nil {
		return nil, err
	}
	return bot, nil
}

func (db *RdsDb) CreateMatch(model *models.Match) error {
	c := db.pool.Get()
	defer c.Close()

	err := SendModel(c, AddSet, getMatchKey(model), CreateMatch(model))
	if err != nil {
		return err
	}
	done := make(map[string]bool)
	for _, bot := range model.Bots {
		ownerPrefix := getPrefix(bot.Owner)
		if !done[ownerPrefix] {
			err := c.Send(addLpush, ownerPrefix+":match-list", model.Uuid)
			if err != nil {
				return err
			}
			done[ownerPrefix] = true
		}
	}
	_, err = flushAndReceive(c)
	return err
}

func (db *RdsDb) UpdateMatch(model *models.Match) error {
	return db.setModelForKey(CreateMatch(model), getMatchKey(model))
}

func (db *RdsDb) GetMatch(matchUuid string) (*Match, error) {
	model := &Match{}
	err := db.getModelForKey(model, getMatchKeyWithUuid(matchUuid))
	if err != nil {
		return nil, err
	} else {
		return model, nil
	}
}

func (db *RdsDb) GetDataMatches(userUuid string, page int, pageSize int) (*Page, error) {
	c := db.pool.Get()
	defer c.Close()
	length, _ := redis.Int(c.Do("LLEN", "user:"+userUuid+":match-list"))
	start := page * pageSize
	end := start + pageSize - 1
	matchUuids, err := redis.Strings(c.Do("LRANGE", "user:"+userUuid+":match-list", start, end))
	if err != nil {
		return nil, err
	}
	matches := make([]interface{}, len(matchUuids))

	for i, matchUuid := range matchUuids {
		rdsMatch := &Match{}
		err = GetModel(c, getMatchKeyWithUuid(matchUuid), rdsMatch)
		if err != nil {
			return nil, err
		}
		matches[i] = rdsMatch
	}
	return &Page{
		matches,
		length,
	}, err
}

func (db *RdsDb) GetMatches(userUuid string, page int, pageSize int) ([]*models.Match, int) {
	c := db.pool.Get()
	defer c.Close()
	length, _ := redis.Int(c.Do("LLEN", "user:"+userUuid+":match-list"))
	start := page * pageSize
	end := start + pageSize - 1
	matchUuids, err := redis.Strings(c.Do("LRANGE", "user:"+userUuid+":match-list", start, end))
	if err != nil {
		return nil, 0
	}
	matches := make([]*models.Match, len(matchUuids))

	for i, matchUuid := range matchUuids {
		rdsMatch := &Match{}
		err = GetModel(c, getMatchKeyWithUuid(matchUuid), rdsMatch)
		if err != nil {
			return nil, 0
		}

		bots := make([]*models.Bot, len(rdsMatch.BotUuids))
		for j, botUuid := range rdsMatch.BotUuids {
			bot := &models.Bot{}
			err = GetModel(c, getBotKeyWithUuid(botUuid), bot)
			if err != nil {
				return nil, 0
			}
			bots[j] = bot
		}
		//noinspection GoStructInitializationWithoutFieldNames
		match := &models.Match{
			rdsMatch.Uuid,
			bots,
			rdsMatch.MapUuid,
			rdsMatch.Winner,
			rdsMatch.Status,
			rdsMatch.Competition,
		}
		matches[i] = match
	}
	return matches, length
}

func (db *RdsDb) CreateBcMap(model *models.BcMap) error {
	c := db.pool.Get()
	defer c.Close()

	err := SendModel(c, AddSet, getBcMapKey(model), model)
	if err != nil {
		return err
	}
	err = c.Send(addLpush, getPrefix(model.Owner)+":map-list", model.Uuid)
	if err != nil {
		return err
	}
	_, err = flushAndReceive(c)
	return err
}

func (db *RdsDb) UpdateBcMap(model *models.BcMap) error {
	return db.setModelForKey(model, getBcMapKey(model))
}

func (db *RdsDb) GetBcMap(uuid string) *models.BcMap {
	model := &models.BcMap{}
	err := db.getModelForKey(model, getBcMapWithUuid(uuid))
	if err != nil {
		return nil
	} else {
		return model
	}
}

func (db *RdsDb) GetBcMaps(userUuid string, page int, pageSize int) ([]*models.BcMap, int) {
	c := db.pool.Get()
	defer c.Close()
	length, _ := redis.Int(c.Do("LLEN", "user:"+userUuid+":map-list"))
	start := page * pageSize
	end := start + pageSize - 1
	bcMapUuids, err := redis.Strings(c.Do("LRANGE", "user:"+userUuid+":map-list", start, end))
	if err != nil {
		return nil, 0
	}
	bcMaps := make([]*models.BcMap, len(bcMapUuids))

	for i, bcMapUuid := range bcMapUuids {
		bcMap := &models.BcMap{}
		err = GetModel(c, getBcMapWithUuid(bcMapUuid), bcMap)
		if err != nil {
			return nil, 0
		}
		bcMaps[i] = bcMap
	}
	return bcMaps, length
}

//utility
func GetModel(c redis.Conn, key string, model interface{}) error {
	bin, err := c.Do("GET", key)
	if err != nil {
		return err
	}
	if bin != nil {
		return json.Unmarshal(bin.([]byte), model)
	}
	return errors.New(fmt.Sprintf("Couldn't find model for key: %q", key))
}

func SendModel(c redis.Conn, action string, key string, model interface{}) error {
	bin, err := json.Marshal(model)
	if err != nil {
		return err
	}
	return c.Send(action, key, bin)
}

func flushAndReceive(c redis.Conn) (interface{}, error) {
	err := c.Flush()
	if err != nil {
		return nil, err
	}
	return c.Receive()
}

func (db *RdsDb) getModelForKey(model interface{}, key string) error {
	c := db.pool.Get()
	defer c.Close()
	return GetModel(c, key, model)
}

func (db *RdsDb) setModelForKey(model interface{}, key string) error {
	c := db.pool.Get()
	defer c.Close()
	err := SendModel(c, AddSet, key, model)
	if err != nil {
		return err
	}
	_, err = flushAndReceive(c)
	return err
}

func getPrefix(c *models.Competitor) string {
	return c.Type.String() + ":" + c.Uuid
}

func getMatchKey(m *models.Match) string {
	return getMatchKeyWithUuid(m.Uuid)
}

func getMatchKeyWithUuid(key string) string {
	return "match:" + key
}

func getBotKey(b *models.Bot) string {
	return getBotKeyWithUuid(b.Uuid)
}

func getBotKeyWithUuid(uuid string) string {
	return "bot:" + uuid
}

func getBcMapKey(m *models.BcMap) string {
	return getBcMapWithUuid(m.Uuid)
}

func getBcMapWithUuid(uuid string) string {
	return "map:" + uuid
}

func (db *RdsDb) Scan(pattern string, run func(redis.Conn, string)) error {
	c := db.pool.Get()
	defer c.Close()
	fmt.Printf("Scanning for %q\n", pattern)
	index := 0
	for true {
		reply, err := redis.Values(c.Do("scan", index, "match", pattern))
		if err != nil {
			return err
		}
		index, err = redis.Int(reply[0], nil)
		if err != nil {
			return err
		}
		keys, err := redis.Strings(reply[1], nil)
		if err != nil {
			return err
		}
		for _, key := range keys {
			run(c, key)
		}
		if index == 0 {
			fmt.Printf("done\n")
			break
		} else {
			fmt.Printf("idx %d complete.\n", index)
		}
	}
	return nil
}

//end utility

//deprecate
func (db *RdsDb) pushModelForKey(model interface{}, key string) error {
	c := db.pool.Get()
	defer c.Close()
	err := SendModel(c, addLpush, key, model)
	if err != nil {
		return err
	}
	_, err = flushAndReceive(c)
	return err
}

//end deprecate