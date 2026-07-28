# PKU Treehole API field notes

This document records the upstream endpoints and response fields used by
PkuHoleStudio. It is not an official API specification.

> All IDs, names, courses, teachers, timestamps, messages and academic records
> below are fictional examples. Never commit cookies, tokens, passwords,
> verification codes, archives, database files, real messages, schedules or
> grades.

## Messages

```text
GET  https://treehole.pku.edu.cn/chapi/api/v3/message/un_read?message_type=int_msg
GET  https://treehole.pku.edu.cn/chapi/api/v3/message/un_read?message_type=sys_msg
GET  https://treehole.pku.edu.cn/chapi/api/v3/message/index?page=1&limit=10&message_type=int_msg
GET  https://treehole.pku.edu.cn/chapi/api/v3/message/index?page=1&limit=10&message_type=sys_msg
POST https://treehole.pku.edu.cn/chapi/api/v3/message/setIntMsgReadByID
POST https://treehole.pku.edu.cn/chapi/api/v3/message/set_read
```

Example request bodies:

```json
{"id": 9000001}
```

```json
{"message_type": "sys_msg"}
```

## Posts and comments

```text
GET  https://treehole.pku.edu.cn/chapi/api/v3/hole/list_comments
GET  https://treehole.pku.edu.cn/chapi/api/v3/hole/one
GET  https://treehole.pku.edu.cn/chapi/api/v3/hole/get
GET  https://treehole.pku.edu.cn/chapi/api/v3/comment/list
POST https://treehole.pku.edu.cn/chapi/api/v3/hole/attention
POST https://treehole.pku.edu.cn/chapi/api/v3/hole/praise
POST https://treehole.pku.edu.cn/chapi/api/v3/hole_draft/add
POST https://treehole.pku.edu.cn/chapi/api/v3/hole/post
POST https://treehole.pku.edu.cn/chapi/api/v3/comment/post
```

Common list parameters:

- `pid`: post ID.
- `page` and `limit`: page number and page size.
- `comment_limit`: number of comments returned with each post.
- `keyword`: search text.
- `label`: upstream tag ID.
- `is_follow`: followed-post filter.
- `kind`: `0` for a regular post and `1` for a reward post.
- `sort`: comment ordering.

Fictional post response shape:

```json
{
  "pid": 9000001,
  "text": "示例树洞正文",
  "type": "text",
  "timestamp": 1780000000,
  "reply": 2,
  "likenum": 3,
  "praise_num": 4,
  "is_follow": 0,
  "is_praise": 0,
  "kind": 0,
  "media_ids": "",
  "tags_ids": "",
  "tags_info": [],
  "identity_info": []
}
```

Field notes:

- `likenum` is the follow count; `praise_num` is the praise count.
- `is_follow` and `is_praise` describe the current account's state.
- `tag` and `label` are not interchangeable; current clients use `label` and
  the tag information collections.
- Several information fields may be an empty array or an object when present,
  so decoders must tolerate both shapes.

Fictional comment response shape:

```json
{
  "cid": 9100001,
  "pid": 9000001,
  "text": "示例评论",
  "timestamp": 1780000100,
  "name_tag": "洞主",
  "media_ids": "",
  "quote": [],
  "is_author": 0,
  "is_lz": 1
}
```

## Media and tags

```text
GET  https://treehole.pku.edu.cn/chapi/api/v3/media/getThumbnail
GET  https://treehole.pku.edu.cn/chapi/api/v3/media/getImageBinary
POST https://treehole.pku.edu.cn/chapi/api/v3/media/uploadImage
GET  https://treehole.pku.edu.cn/chapi/api/v3/tags/tree
```

Media upload uses a multipart form. Do not place real upload responses or
storage paths in this repository.

## Course schedule

```text
GET https://treehole.pku.edu.cn/chapi/api/course/table
```

Fictional response shape:

```json
{
  "code": 20000,
  "data": {
    "kb": [
      {
        "timeNum": "第一节",
        "mon": {
          "courseName": "示例课程（教师：示例教师；1-16 周）",
          "parity": "",
          "sty": "background-color: aquamarine"
        }
      }
    ],
    "remark": ""
  },
  "message": "success",
  "success": true,
  "timestamp": 0
}
```

## Scores

```text
GET https://treehole.pku.edu.cn/chapi/api/course/score_v2
```

Fictional response shape:

```json
{
  "code": 20000,
  "data": {
    "score": {
      "success": true,
      "xslb": "bks",
      "jbxx": {
        "xh": "0000000000",
        "xm": "示例学生",
        "xsmc": "示例学院",
        "xsyw": "Example School",
        "zymc": "示例专业",
        "zyywmc": "Example Major",
        "grade": "2025",
        "zxnj": "2025",
        "xjzt": "在校生"
      },
      "cjxx": [
        {
          "xnd": "25-26",
          "xq": "1",
          "kch": "EXAMPLE001",
          "kcmc": "示例课程",
          "ywmc": "Example Course",
          "xf": "3",
          "xqcj": "95",
          "cjjlfs": "百分制",
          "kclbmc": "任选",
          "skjsxm": "示例教师"
        }
      ]
    }
  },
  "message": "success",
  "success": true,
  "timestamp": 0
}
```
