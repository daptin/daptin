SELECT
  usergroup.name,
  usergroup.id,
  usergroup.created_at,
  usergroup.updated_at,
  usergroup.deleted_at,
  usergroup.reference_id,
  usergroup.permission,
  usergroup.status
FROM usergroup
  JOIN blog_has_usergroup j1 ON j1.blog_id = blog.id
  JOIN usergroup ON j1.usergroup_id = usergroup.id
  JOIN post_has_usergroup j1 ON j1.post_id = post.id
  JOIN usergroup ON j1.usergroup_id = usergroup.id
  JOIN comment_has_usergroup j1 ON j1.comment_id = comment.id
  JOIN usergroup ON j1.usergroup_id = usergroup.id
  JOIN world_has_usergroup j1 ON j1.world_id = world.id
  JOIN usergroup ON j1.usergroup_id = usergroup.id
  JOIN user_has_usergroup j1 ON j1.user_id = user.id
  JOIN usergroup ON j1.usergroup_id = usergroup.id
  JOIN action_has_usergroup j1 ON j1.action_id = action.id
  JOIN usergroup ON j1.usergroup_id = usergroup.id
WHERE usergroup.deleted_at IS NULL AND blog.reference_id IN (?) AND post.reference_id IN (NULL) AND
      comment.reference_id IN (NULL) AND world.reference_id IN (NULL) AND user.reference_id IN (NULL) AND
      action.reference_id IN (NULL)
LIMIT 10 OFFSET 0;