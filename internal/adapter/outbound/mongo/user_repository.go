package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/poppsfs4za/backend-challenge/internal/core/domain"
	"github.com/poppsfs4za/backend-challenge/internal/core/port"
)

// userDocument คือรูปร่างข้อมูลในมุมของ MongoDB
// แยกจาก domain.User โดยตั้งใจ — แกนกลางจะได้ไม่มี bson tag ปนเปื้อน
type userDocument struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	Email     string        `bson:"email"`
	Password  string        `bson:"password"`
	CreatedAt time.Time     `bson:"created_at"`
}

func (d userDocument) toDomain() *domain.User {
	return &domain.User{
		ID:        d.ID.Hex(),
		Name:      d.Name,
		Email:     d.Email,
		Password:  d.Password,
		CreatedAt: d.CreatedAt,
	}
}

type UserRepository struct {
	coll *mongo.Collection
}

var _ port.UserRepository = (*UserRepository)(nil)

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{coll: db.Collection("users")}
}

// EnsureIndexes สร้าง unique index บน email ตามโจทย์ข้อ 1
func (r *UserRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_email"),
	})
	return err
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	doc := userDocument{
		Name:      u.Name,
		Email:     u.Email,
		Password:  u.Password,
		CreatedAt: u.CreatedAt,
	}

	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrEmailAlreadyExists
		}
		return err
	}

	// เขียน ID ที่ Mongo สร้างให้ กลับเข้า entity
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		u.ID = oid.Hex()
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrInvalidID
	}

	var doc userDocument
	if err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: oid}}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return doc.toDomain(), nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var doc userDocument
	if err := r.coll.FindOne(ctx, bson.D{{Key: "email", Value: email}}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return doc.toDomain(), nil
}

func (r *UserRepository) List(ctx context.Context) ([]*domain.User, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cur, err := r.coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs []userDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}

	users := make([]*domain.User, 0, len(docs))
	for _, d := range docs {
		users = append(users, d.toDomain())
	}
	return users, nil
}

func (r *UserRepository) Update(ctx context.Context, id, name, email string) (*domain.User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrInvalidID
	}

	// สร้าง $set แบบ dynamic — แก้เฉพาะ field ที่ส่งมา
	set := bson.D{}
	if name != "" {
		set = append(set, bson.E{Key: "name", Value: name})
	}
	if email != "" {
		set = append(set, bson.E{Key: "email", Value: email})
	}
	if len(set) == 0 {
		return r.GetByID(ctx, id)
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var doc userDocument
	err = r.coll.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: oid}},
		bson.D{{Key: "$set", Value: set}},
		opts,
	).Decode(&doc)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		if mongo.IsDuplicateKeyError(err) {
			return nil, domain.ErrEmailAlreadyExists
		}
		return nil, err
	}
	return doc.toDomain(), nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return domain.ErrInvalidID
	}

	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.D{})
}
