package main

import (
	"errors"
	"fmt"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/library"
)

// CollectionInfo describes a collection for the frontend.
type CollectionInfo struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Images int    `json:"images"`
}

// ListCollections returns all collections by name, with image counts.
func (a *App) ListCollections() (list []CollectionInfo, err error) {
	defer a.recoverPanic("ListCollections", &err)

	cols, err := a.library.Collections(a.ctx)
	if err != nil {
		return nil, apperr.New(apperr.Unavailable, "Collections could not be loaded.", err)
	}
	list = make([]CollectionInfo, len(cols))
	for i, c := range cols {
		list[i] = CollectionInfo{ID: c.ID, Name: c.Name, Images: c.Images}
	}
	return list, nil
}

// CreateCollection creates an empty collection.
func (a *App) CreateCollection(name string) (info CollectionInfo, err error) {
	defer a.recoverPanic("CreateCollection", &err)

	c, err := a.library.CreateCollection(a.ctx, name)
	if err != nil {
		return CollectionInfo{}, collectionError(err, "The collection could not be created.")
	}
	return CollectionInfo{ID: c.ID, Name: c.Name}, nil
}

// RenameCollection changes a collection's name.
func (a *App) RenameCollection(id int64, name string) (err error) {
	defer a.recoverPanic("RenameCollection", &err)

	if err := a.library.RenameCollection(a.ctx, id, name); err != nil {
		return collectionError(err, "The collection could not be renamed.")
	}
	return nil
}

// DeleteCollection removes a collection. Its images stay in the library.
func (a *App) DeleteCollection(id int64) (err error) {
	defer a.recoverPanic("DeleteCollection", &err)

	if err := a.library.DeleteCollection(a.ctx, id); err != nil {
		return collectionError(err, "The collection could not be deleted.")
	}
	return nil
}

// AddToCollection puts a library image in a collection.
func (a *App) AddToCollection(collectionID, imageID int64) (err error) {
	defer a.recoverPanic("AddToCollection", &err)

	if err := a.library.AddToCollection(a.ctx, collectionID, imageID); err != nil {
		return collectionError(err, "The image could not be added to the collection.")
	}
	return nil
}

// RemoveFromCollection takes an image out of a collection; the image stays
// in the library.
func (a *App) RemoveFromCollection(collectionID, imageID int64) (err error) {
	defer a.recoverPanic("RemoveFromCollection", &err)

	if err := a.library.RemoveFromCollection(a.ctx, collectionID, imageID); err != nil {
		return collectionError(err, "The image could not be removed from the collection.")
	}
	return nil
}

// ImageCollections returns the IDs of the collections an image belongs to.
func (a *App) ImageCollections(imageID int64) (ids []int64, err error) {
	defer a.recoverPanic("ImageCollections", &err)

	ids, err = a.library.ImageCollections(a.ctx, imageID)
	if err != nil {
		return nil, apperr.New(apperr.Unavailable, "Collections could not be loaded.", err)
	}
	if ids == nil {
		ids = []int64{} // an empty list, not null, for the frontend
	}
	return ids, nil
}

// collectionError maps library errors to messages; fallback describes
// what failed for anything unexpected.
func collectionError(err error, fallback string) error {
	switch {
	case errors.Is(err, library.ErrCollectionExists):
		return apperr.New(apperr.InvalidInput, "A collection with this name already exists.", err)
	case errors.Is(err, library.ErrInvalidName):
		return apperr.New(apperr.InvalidInput,
			fmt.Sprintf("Collection names must be 1 to %d characters long.", library.MaxCollectionName), err)
	case errors.Is(err, library.ErrNotFound):
		return apperr.New(apperr.InvalidInput, "This collection or image no longer exists.", err)
	}
	return apperr.New(apperr.Unavailable, fallback, err)
}
