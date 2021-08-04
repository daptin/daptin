# Streams

Streams are complimentary to [actions](/actions/actions). Think of streams as views in SQL. A stream is basically one entity + set of transformations and filters on the entity. Streams are read-only and exposed with similar semantics of that of entities. Daptin will expose JSONAPI for each stream just like it does for entities.

Here is an example of a stream which exposes list of completed todos only

```
{
		StreamName:     "transformed_user",
		RootEntityName: "todo",
		Columns: []api2go.ColumnInfo{          // List of columns in this stream
			{
				Name:       "transformed_todo_title",  
				ColumnType: "label",
			},
			{
				Name:       "completed_on",
				ColumnType: "datetime",
			},
		},
		QueryParams: QueryParams{
			"Filter": "completed=true",
			"Select": "title,deadline",
		},
		Transformations: []Transformation{
			{
				Operation: "select",
				Attributes: map[string]interface{}{
					"columns": []string{"title", "deadline"},
				},
			},
			{
				Operation: "rename",
				Attributes: map[string]interface{}{
					"oldName": "title",
					"newName": "transformed_todo_title",
				},
			},
			{
				Operation: "rename",
				Attributes: map[string]interface{}{
					"oldName": "deadline",
					"newName": "completed_on",
				},
			},
		},
}	
```

Daptin uses the library [kniren/gota](github.com/kniren/gota/dataframe) to systematically specific list of transformations which are applied to the original data stream.