# API

## check-health同时查看未读message
https://treehole.pku.edu.cn/chapi/api/v3/message/un_read?message_type=int_msg
https://treehole.pku.edu.cn/chapi/api/v3/message/un_read?message_type=sys_msg

示例：unread_messages.json

## 获取消息
https://treehole.pku.edu.cn/chapi/api/v3/message/index?page=1&limit=10&message_type=int_msg
https://treehole.pku.edu.cn/chapi/api/v3/message/index?page=1&limit=10&message_type=sys_msg

示例：int_messages.json / sys_messages.json

## 获取树洞列表及评论
https://treehole.pku.edu.cn/chapi/api/v3/hole/list_comments?pid=7999240&page=1&limit=10&comment_limit=10&keyword=zkc&label=1&is_follow=1&kind=1&comment_stream=1

kind表示树洞类型，0为普通树洞，1为悬赏树洞，is_follow表示是否关注树洞, label表示标签的编号，comment_stream功能暂不明确，固定带着

示例：list_comments.json

## 获取单个帖子
https://treehole.pku.edu.cn/chapi/api/v3/hole/one?pid=8110393&comment_stream=1

示例：one.json

## 获取评论
https://treehole.pku.edu.cn/chapi/api/v3/comment/list?pid=8127902&page=1&limit=10&sort=0&comment_stream=1

示例：comment.json

## 关注/取消关注（同一接口）
POST https://treehole.pku.edu.cn/chapi/api/v3/hole/attention
eg: {"pid":8141313}

示例：attention_0.json / attention_1.json

## 点赞/取消赞（同一接口）
POST https://treehole.pku.edu.cn/chapi/api/v3/hole/praise
eg: {"pid":8141313}

示例：praise.json （返回数据相同）

## 获取帖子信息（web中在点赞/取消赞/关注/取消关注之后会调用）
https://treehole.pku.edu.cn/chapi/api/v3/hole/get?pid=8141313

示例：get.json

## 获取图片缩略图(media_id/pid)
https://treehole.pku.edu.cn/chapi/api/v3/media/getThumbnail?id=16504
https://treehole.pku.edu.cn/chapi/api/v3/media/getThumbnail?pid=8141302

## 获取图片原图(media_id/pid)
https://treehole.pku.edu.cn/chapi/api/v3/media/getImageBinary?id=16504
https://treehole.pku.edu.cn/chapi/api/v3/media/getImageBinary?pid=8141306

## 获取tag列表
https://treehole.pku.edu.cn/chapi/api/v3/tags/tree

示例：tags.json

## 发布前保存草稿
POST https://treehole.pku.edu.cn/chapi/api/v3/hole_draft/add
eg: {
    "type": "text",
    "kind": 0,
    "reward_cost": 1,
    "text": "test",
    "identity_show": 0,
    "identity_type": "",
    "exclusive_id_id": "",
    "fold": 0,
    "mailbox": 0,
    "tags_ids": "",
    "media_ids": ""
}

## 发布树洞
POST https://treehole.pku.edu.cn/chapi/api/v3/hole/post
eg: {
    "type": "text",
    "kind": 0,
    "reward_cost": 1,
    "text": "test",
    "identity_show": 0,
    "identity_type": "",
    "exclusive_id_id": "",
    "fold": 0,
    "mailbox": 0,
    "tags_ids": "",
    "media_ids": ""
},
{
    "type": "image",
    "kind": 0,
    "reward_cost": 1,
    "text": "test",
    "identity_show": 0,
    "identity_type": "",
    "exclusive_id_id": "",
    "fold": 0,
    "mailbox": 0,
    "tags_ids": "",
    "media_ids": "35802,35803"
}

## 发布评论
POST https://treehole.pku.edu.cn/chapi/api/v3/comment/post
eg: {
    "pid": 8139378,
    "comment_id": "",
    "text": "test",
    "media_ids": "",
    "identity_show": 0,
    "identity_type": ""
}

## 上传图片供树洞及评论用
POST https://treehole.pku.edu.cn/chapi/api/v3/media/uploadImage
表单传二进制，
返回eg:
{
    "code": 20000,
    "data": {
        "id": 35803,
        "url": "2026/6/18/1aab17d84a4464f957ff750b3f87bbc2183fba28_1920x1440.jpg"
    },
    "message": "success",
    "success": true,
    "timestamp": 1781714006
}

## 获取课表

GET https://treehole.pku.edu.cn/chapi/api/getCoursetable_v2
{
    "code": 20000,
    "data": {
        "flag": "success",
        "course": [
            {
                "fri": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "timeNum": "第一节",
                "tue": {
                    "courseName": "<font color = 'red'><b>量子力学(主)<br>上课信息：1-15周 单周 二教411  教师：舒菁 备注：2024级及以后适用，先修理论物理基础I&II。<br>考试信息：20260618 星期四 上午 二教411<br>量子力学习题(主)<br>上课信息：1-15周 双周 二教410  教师：舒菁 备注：2024级（含）以后适用，3学分量子力学习题课。<br>考试信息： </b></font>",
                    "parity": "",
                    "sty": "white"
                },
                "wed": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                }
            },
            {
                "fri": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "timeNum": "第二节",
                "tue": {
                    "courseName": "<font color = 'red'><b>量子力学(主)<br>上课信息：1-15周 单周 二教411  教师：舒菁 备注：2024级及以后适用，先修理论物理基础I&II。<br>考试信息：20260618 星期四 上午 二教411<br>量子力学习题(主)<br>上课信息：1-15周 双周 二教410  教师：舒菁 备注：2024级（含）以后适用，3学分量子力学习题课。<br>考试信息： </b></font>",
                    "parity": "",
                    "sty": "white"
                },
                "wed": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                }
            },
            {
                "fri": {
                    "courseName": "固体物理学(主)<br>上课信息：1-15周 每周 三教507  教师：冯济 备注：（卓越班）允许替代2024级00432512固体物理学（3学分）。<br>考试信息：20260621 星期七 下午 二教505",
                    "parity": "",
                    "sty": "background-color: lightgrey"
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "量子力学(主)<br>上课信息：1-15周 每周 二教411  教师：舒菁 备注：2024级及以后适用，先修理论物理基础I&II。<br>考试信息：20260618 星期四 上午 二教411",
                    "parity": "",
                    "sty": "background-color: lightcyan"
                },
                "timeNum": "第三节",
                "tue": {
                    "courseName": "广义相对论(主)<br>上课信息：1-15周 每周   教师：阮善明 备注：研本合上；上课地点：理教309<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: lightblue"
                },
                "wed": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                }
            },
            {
                "fri": {
                    "courseName": "固体物理学(主)<br>上课信息：1-15周 每周 三教507  教师：冯济 备注：（卓越班）允许替代2024级00432512固体物理学（3学分）。<br>考试信息：20260621 星期七 下午 二教505",
                    "parity": "",
                    "sty": "background-color: lightgrey"
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "量子力学(主)<br>上课信息：1-15周 每周 二教411  教师：舒菁 备注：2024级及以后适用，先修理论物理基础I&II。<br>考试信息：20260618 星期四 上午 二教411",
                    "parity": "",
                    "sty": "background-color: lightcyan"
                },
                "timeNum": "第四节",
                "tue": {
                    "courseName": "广义相对论(主)<br>上课信息：1-15周 每周   教师：阮善明 备注：研本合上；上课地点：理教309<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: lightblue"
                },
                "wed": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                }
            },
            {
                "fri": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "广义相对论(主)<br>上课信息：1-15周 每周   教师：阮善明 备注：研本合上；上课地点：理教309<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: lightblue"
                },
                "timeNum": "第五节",
                "tue": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "wed": {
                    "courseName": "物理学前沿中的精密测量(主)<br>上课信息：1-15周 每周   教师：边珂,江颖,林熙,刘开辉,马文君,王国庆,肖云峰,杨振伟 备注：研本合上，卓越荣誉选修课，两个班择一，具体实验周次待上课后安排。<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: aquamarine"
                }
            },
            {
                "fri": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "广义相对论(主)<br>上课信息：1-15周 每周   教师：阮善明 备注：研本合上；上课地点：理教309<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: lightblue"
                },
                "timeNum": "第六节",
                "tue": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "wed": {
                    "courseName": "物理学前沿中的精密测量(主)<br>上课信息：1-15周 每周   教师：边珂,江颖,林熙,刘开辉,马文君,王国庆,肖云峰,杨振伟 备注：研本合上，卓越荣誉选修课，两个班择一，具体实验周次待上课后安排。<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: aquamarine"
                }
            },
            {
                "fri": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "timeNum": "第七节",
                "tue": {
                    "courseName": "固体物理学(主)<br>上课信息：1-15周 每周 三教507  教师：冯济 备注：（卓越班）允许替代2024级00432512固体物理学（3学分）。<br>考试信息：20260621 星期七 下午 二教505",
                    "parity": "",
                    "sty": "background-color: lightgrey"
                },
                "wed": {
                    "courseName": "物理学前沿中的精密测量(主)<br>上课信息：1-15周 每周   教师：边珂,江颖,林熙,刘开辉,马文君,王国庆,肖云峰,杨振伟 备注：研本合上，卓越荣誉选修课，两个班择一，具体实验周次待上课后安排。<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: aquamarine"
                }
            },
            {
                "fri": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "timeNum": "第八节",
                "tue": {
                    "courseName": "固体物理学(主)<br>上课信息：1-15周 每周 三教507  教师：冯济 备注：（卓越班）允许替代2024级00432512固体物理学（3学分）。<br>考试信息：20260621 星期七 下午 二教505",
                    "parity": "",
                    "sty": "background-color: lightgrey"
                },
                "wed": {
                    "courseName": "物理学前沿中的精密测量(主)<br>上课信息：1-15周 每周   教师：边珂,江颖,林熙,刘开辉,马文君,王国庆,肖云峰,杨振伟 备注：研本合上，卓越荣誉选修课，两个班择一，具体实验周次待上课后安排。<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: aquamarine"
                }
            },
            {
                "fri": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "timeNum": "第九节",
                "tue": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "wed": {
                    "courseName": "物理学前沿中的精密测量(主)<br>上课信息：1-15周 每周   教师：边珂,江颖,林熙,刘开辉,马文君,王国庆,肖云峰,杨振伟 备注：研本合上，卓越荣誉选修课，两个班择一，具体实验周次待上课后安排。<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: aquamarine"
                }
            },
            {
                "fri": {
                    "courseName": "地震概论(主)<br>上课信息：1-15周 每周 理教108  教师：赵克常 <br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: lightsalmon"
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "毛泽东思想和中国特色社会主义理论体系概论(主)<br>上课信息：1-15周 每周 理教102  教师：程美东,贺大兴,孙蚌珠,孙超 <br>考试信息：20260621 星期七 晚上 ",
                    "parity": "",
                    "sty": "background-color: lightseagreen"
                },
                "timeNum": "第十节",
                "tue": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "wed": {
                    "courseName": "物理学前沿中的精密测量(主)<br>上课信息：1-15周 每周   教师：边珂,江颖,林熙,刘开辉,马文君,王国庆,肖云峰,杨振伟 备注：研本合上，卓越荣誉选修课，两个班择一，具体实验周次待上课后安排。<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: aquamarine"
                }
            },
            {
                "fri": {
                    "courseName": "地震概论(主)<br>上课信息：1-15周 每周 理教108  教师：赵克常 <br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: lightsalmon"
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "毛泽东思想和中国特色社会主义理论体系概论(主)<br>上课信息：1-15周 每周 理教102  教师：程美东,贺大兴,孙蚌珠,孙超 <br>考试信息：20260621 星期七 晚上 ",
                    "parity": "",
                    "sty": "background-color: lightseagreen"
                },
                "timeNum": "第十一节",
                "tue": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "wed": {
                    "courseName": "物理学前沿中的精密测量(主)<br>上课信息：1-15周 每周   教师：边珂,江颖,林熙,刘开辉,马文君,王国庆,肖云峰,杨振伟 备注：研本合上，卓越荣誉选修课，两个班择一，具体实验周次待上课后安排。<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: aquamarine"
                }
            },
            {
                "fri": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "mon": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sat": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "sun": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "thu": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "timeNum": "第十二节",
                "tue": {
                    "courseName": "",
                    "parity": "",
                    "sty": ""
                },
                "wed": {
                    "courseName": "物理学前沿中的精密测量(主)<br>上课信息：1-15周 每周   教师：边珂,江颖,林熙,刘开辉,马文君,王国庆,肖云峰,杨振伟 备注：研本合上，卓越荣誉选修课，两个班择一，具体实验周次待上课后安排。<br>考试信息： ",
                    "parity": "",
                    "sty": "background-color: aquamarine"
                }
            }
        ],
        "remark": ""
    },
    "message": "success",
    "success": true,
    "timestamp": 1781716083
}

## 查询成绩

GET https://treehole.pku.edu.cn/chapi/api/course/score_v2
{
    "code": 20000,
    "data": {
        "score": {
            "success": true,
            "xslb": "bks",
            "jbxx": {
                "xh": "2400011461",
                "xsyw": "School of Physics",
                "xm": "张景天",
                "xsmc": "物理学院",
                "zyywmc": "Physics",
                "grade": "2024",
                "xjzt": "在校生",
                "xmpy": "Zhang Jingtian",
                "zxnj": "2024",
                "zymc": "物理学"
            },
            "cjxx": [
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2526200432297_12103",
                    "jxbh": "1",
                    "kctxm": "04",
                    "xqcj": "W",
                    "xq": "2",
                    "ywmc": "Statistical methods in experimental physics",
                    "skjsxm": "2106176311-杨振伟$物理学院$教授",
                    "skjszgh": "2106176311(00004)",
                    "xnd": "25-26",
                    "kch": "00432297",
                    "bkcjbh": "zqtkcj2026050000090967",
                    "kclbmc": "任选",
                    "xndxqpx": "2025-262",
                    "kcmc": "实验物理中的统计方法",
                    "xndpx": "2025-26",
                    "xf": "3",
                    "cjjlfs": "",
                    "kclb": "30",
                    "kctx": "专业任选"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2526100137975_17251",
                    "jxbh": "1",
                    "kctxm": "06",
                    "xqcj": "99",
                    "xq": "1",
                    "ywmc": "Music and Mathematics",
                    "skjsxm": "0006156002-王杰$党办校办$教授",
                    "skjszgh": "0006156002(00601)",
                    "xnd": "25-26",
                    "kch": "00137975",
                    "bkcjbh": "bkcj2026010002468543",
                    "kclbmc": "通选课",
                    "xndxqpx": "2025-261",
                    "kcmc": "音乐与数学",
                    "xndpx": "2025-26",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "32",
                    "kctx": "通选课"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2526100431648_10320",
                    "jxbh": "1",
                    "kctxm": "03",
                    "xqcj": "96",
                    "xq": "1",
                    "ywmc": "Statistical mechanics",
                    "skjsxm": "2006189107-黄华卿$物理学院$助理教授",
                    "skjszgh": "2006189107(00004)",
                    "xnd": "25-26",
                    "kch": "00431648",
                    "bkcjbh": "bkcj2026010002500930",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2025-261",
                    "kcmc": "统计力学",
                    "xndpx": "2025-26",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2526100432109_14056",
                    "jxbh": "1",
                    "kctxm": "03",
                    "xqcj": "96",
                    "xq": "1",
                    "ywmc": "Methods of Mathematical Physics (2)",
                    "skjsxm": "0006165543-李定平$物理学院$教授",
                    "skjszgh": "0006165543(00004)",
                    "xnd": "25-26",
                    "kch": "00432109",
                    "bkcjbh": "bkcj2026010002480439",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2025-261",
                    "kcmc": "数学物理方法 (下)",
                    "xndpx": "2025-26",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2526100432213_16459",
                    "jxbh": "1",
                    "kctxm": "03",
                    "xqcj": "97",
                    "xq": "1",
                    "ywmc": "Classical Electrodynamics",
                    "skjsxm": "2206188142-徐新路$物理学院$助理教授",
                    "skjszgh": "2206188142(00004)",
                    "xnd": "25-26",
                    "kch": "00432213",
                    "bkcjbh": "bkcj2026010002517013",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2025-261",
                    "kcmc": "电动力学",
                    "xndpx": "2025-26",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2526101035390_10068",
                    "jxbh": "1",
                    "kctxm": "07",
                    "xqcj": "合格",
                    "xq": "1",
                    "ywmc": "Boya Science Lectures",
                    "skjsxm": "1006172506-高毅勤$化学学院$教授",
                    "skjszgh": "1006172506(00010)",
                    "xnd": "25-26",
                    "kch": "01035390",
                    "bkcjbh": "bkcj2026010002536032",
                    "kclbmc": "全校任选",
                    "xndxqpx": "2025-261",
                    "kcmc": "博雅理学讲堂",
                    "xndpx": "2025-26",
                    "xf": "1",
                    "cjjlfs": "合格制",
                    "kclb": "31",
                    "kctx": "全校公选课"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2526101130871_26550",
                    "jxbh": "2",
                    "kctxm": "06",
                    "xqcj": "合格",
                    "xq": "1",
                    "ywmc": "Human Sex, Reproduction and Health",
                    "skjsxm": "0006173110-姚锦仙$生命学院$副教授",
                    "skjszgh": "0006173110(00011)",
                    "xnd": "25-26",
                    "kch": "01130871",
                    "bkcjbh": "bkcj2026010002467414",
                    "kclbmc": "通选课",
                    "xndxqpx": "2025-261",
                    "kcmc": "人类的性、生育与健康",
                    "xndpx": "2025-26",
                    "xf": "2",
                    "cjjlfs": "百分制",
                    "kclb": "32",
                    "kctx": "通选课"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2526102035210_13534",
                    "jxbh": "1",
                    "kctxm": "07",
                    "xqcj": "合格",
                    "xq": "1",
                    "ywmc": "Boya Lectures of Humanities",
                    "skjsxm": "2306186190-胡鸿$历史系$教授,0006164342-韩林合$哲学系$教授,0006169489-李维$历史系$教授,0006167204-张帆$历史系$教授,2206184215-李锋$考古系$长聘副教授,2006189119-李云河$考古系$助理教授,0006163522-张辉$中文系$教授,0006162058-郭锐$中文系$教授,1106165623-郑开$哲学系$教授",
                    "skjszgh": "2306186190(00021),0006164342(00023),0006169489(00021),0006167204(00021),2206184215(00022),2006189119(00022),0006163522(00020),0006162058(00020),1106165623(00023)",
                    "xnd": "25-26",
                    "kch": "02035210",
                    "bkcjbh": "bkcj2026010002506487",
                    "kclbmc": "全校任选",
                    "xndxqpx": "2025-261",
                    "kcmc": "博雅人文讲堂",
                    "xndpx": "2025-26",
                    "xf": "1",
                    "cjjlfs": "合格制",
                    "kclb": "31",
                    "kctx": "全校公选课"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2526104031762_14155",
                    "jxbh": "1",
                    "kctxm": "08",
                    "xqcj": "88",
                    "xq": "1",
                    "ywmc": "Introduction to Xi Jinping Thought on Socialism with Chinese Characteristics for a New Era ",
                    "skjsxm": "0006163221-孙蚌珠$马克思学院$教授,2206184217-董彪$习近平新时代中国特色社会主义思想研究院$助理教授,2506184225-王娜$马克思学院$长聘副教授",
                    "skjszgh": "0006163221(00040),2206184217(00221),2506184225(00040)",
                    "xnd": "25-26",
                    "kch": "04031762",
                    "bkcjbh": "bkcj2026010002455697",
                    "kclbmc": "全校必修",
                    "xndxqpx": "2025-261",
                    "kcmc": "习近平新时代中国特色社会主义思想概论",
                    "xndpx": "2025-26",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "11",
                    "kctx": "思想政治"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2526104130210_15011",
                    "jxbh": "1",
                    "kctxm": "11",
                    "xqcj": "91",
                    "xq": "1",
                    "ywmc": "Baseball",
                    "skjsxm": "1706191051-焦晨曦$体教$讲师",
                    "skjszgh": "1706191051(00041)",
                    "xnd": "25-26",
                    "kch": "04130210",
                    "bkcjbh": "bkcj2026010002438471",
                    "kclbmc": "全校必修",
                    "xndxqpx": "2025-261",
                    "kcmc": "棒、垒球",
                    "xndpx": "2025-26",
                    "xf": "1",
                    "cjjlfs": "百分制",
                    "kclb": "11",
                    "kctx": "体育"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425200132512_41667",
                    "jxbh": "4",
                    "kctxm": "03",
                    "xqcj": "94",
                    "xq": "2",
                    "ywmc": "Advanced Mathematics A (no. 2)",
                    "skjsxm": "0006172312-吴金彪$数学学院$副教授",
                    "skjszgh": "0006172312(00001)",
                    "xnd": "24-25",
                    "kch": "00132512",
                    "bkcjbh": "bkcj2025060002337749",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2024-252",
                    "kcmc": "高等数学A（二）",
                    "xndpx": "2024-25",
                    "xf": "5",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425200431142_56933",
                    "jxbh": "5",
                    "kctxm": "03",
                    "xqcj": "EX",
                    "xq": "2",
                    "ywmc": "Thermal Physics",
                    "skjsxm": "",
                    "skjszgh": "",
                    "xnd": "24-25",
                    "kch": "00431142",
                    "bkcjbh": "bkcj2025060002330839",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2024-252",
                    "kcmc": "热学",
                    "xndpx": "2024-25",
                    "xf": "2",
                    "cjjlfs": "合格制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425200432002_16375",
                    "jxbh": "1",
                    "kctxm": "03",
                    "xqcj": "93",
                    "xq": "2",
                    "ywmc": "Fundamentals of Theoretical Physics II",
                    "skjsxm": "0006166367-刘川$物理学院$教授",
                    "skjszgh": "0006166367(00004)",
                    "xnd": "24-25",
                    "kch": "00432002",
                    "bkcjbh": "bkcj2025060002383954",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2024-252",
                    "kcmc": "理论物理基础II",
                    "xndpx": "2024-25",
                    "xf": "4",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425200432108_13900",
                    "jxbh": "1",
                    "kctxm": "03",
                    "xqcj": "95.5",
                    "xq": "2",
                    "ywmc": "Methods of Mathematical Physics (1)",
                    "skjsxm": "2306183222-朱华星$物理学院$教授",
                    "skjszgh": "2306183222(00004)",
                    "xnd": "24-25",
                    "kch": "00432108",
                    "bkcjbh": "bkcj2025060002368576",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2024-252",
                    "kcmc": "数学物理方法 (上)",
                    "xndpx": "2024-25",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425200432211_15181",
                    "jxbh": "1",
                    "kctxm": "03",
                    "xqcj": "92",
                    "xq": "2",
                    "ywmc": "Theoretical Mechanics",
                    "skjsxm": "1806185138-赵鹏巍$物理学院$长聘副教授",
                    "skjszgh": "1806185138(00004)",
                    "xnd": "24-25",
                    "kch": "00432211",
                    "bkcjbh": "bkcj2025060002379136",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2024-252",
                    "kcmc": "理论力学",
                    "xndpx": "2024-25",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425200434102_14512",
                    "jxbh": "1",
                    "kctxm": "04",
                    "xqcj": "合格",
                    "xq": "2",
                    "ywmc": "Distinguished Lecture Series in Physics: When Excellence Meets (II)",
                    "skjsxm": "1806163715-高原宁$物理学院$教授,0006178256-孙琰$物理学院$助理研究员",
                    "skjszgh": "1806163715(00004),0006178256(00004)",
                    "xnd": "24-25",
                    "kch": "00434102",
                    "bkcjbh": "bkcj2025060002347619",
                    "kclbmc": "任选",
                    "xndxqpx": "2024-252",
                    "kcmc": "物理卓越计划讲堂：名师面对面（二）",
                    "xndpx": "2024-25",
                    "xf": "1",
                    "cjjlfs": "合格制",
                    "kclb": "30",
                    "kctx": "专业任选"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425203835995_10773",
                    "jxbh": "1",
                    "kctxm": "09",
                    "xqcj": "87",
                    "xq": "2",
                    "ywmc": "Academic English Reading",
                    "skjsxm": "2306390614-方舒琼$外国语学院$助理研究员",
                    "skjszgh": "2306390614(00039)",
                    "xnd": "24-25",
                    "kch": "03835995",
                    "bkcjbh": "bkcj2025060002408272",
                    "kclbmc": "全校必修",
                    "xndxqpx": "2024-252",
                    "kcmc": "学术英语阅读",
                    "xndpx": "2024-25",
                    "xf": "2",
                    "cjjlfs": "百分制",
                    "kclb": "11",
                    "kctx": "大学英语"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425204031661138506",
                    "jxbh": "13",
                    "kctxm": "08",
                    "xqcj": "87",
                    "xq": "2",
                    "ywmc": "Outline of Chinese Modern History",
                    "skjsxm": "2206186182-贾凯$马克思学院$预聘副教授",
                    "skjszgh": "2206186182(00040)",
                    "xnd": "24-25",
                    "kch": "04031661",
                    "bkcjbh": "bkcj2025060002330986",
                    "kclbmc": "全校必修",
                    "xndxqpx": "2024-252",
                    "kcmc": "中国近现代史纲要",
                    "xndpx": "2024-25",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "11",
                    "kctx": "思想政治"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425204130050_60862",
                    "jxbh": "6",
                    "kctxm": "11",
                    "xqcj": "96",
                    "xq": "2",
                    "ywmc": "Table Tennis",
                    "skjsxm": "1306185099-周正卿$体教$教学副教授",
                    "skjszgh": "1306185099(00041)",
                    "xnd": "24-25",
                    "kch": "04130050",
                    "bkcjbh": "bkcj2025060002381582",
                    "kclbmc": "全校必修",
                    "xndxqpx": "2024-252",
                    "kcmc": "乒乓球",
                    "xndpx": "2024-25",
                    "xf": "1",
                    "cjjlfs": "百分制",
                    "kclb": "11",
                    "kctx": "体育"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425204831420142897",
                    "jxbh": "14",
                    "kctxm": "02",
                    "xqcj": "100",
                    "xq": "2",
                    "ywmc": "Data Structure and Algorithm (B)",
                    "skjsxm": "0006173231-闫宏飞$计算机学院$副教授",
                    "skjszgh": "0006173231(00101)",
                    "xnd": "24-25",
                    "kch": "04831420",
                    "bkcjbh": "bkcj2025060002329282",
                    "kclbmc": "全校必修",
                    "xndxqpx": "2024-252",
                    "kcmc": "数据结构与算法 (B)",
                    "xndpx": "2024-25",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "11",
                    "kctx": "理科生必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425261130040_12877",
                    "jxbh": "1",
                    "kctxm": "08",
                    "xqcj": "合格",
                    "xq": "2",
                    "ywmc": "Social practice and service learning, Part II",
                    "skjsxm": "",
                    "skjszgh": "",
                    "xnd": "24-25",
                    "kch": "61130040",
                    "bkcjbh": "bkcj24-252025090002433577",
                    "kclbmc": "全校必修",
                    "xndxqpx": "2024-252",
                    "kcmc": "思想政治实践（下）",
                    "xndpx": "2024-25",
                    "xf": "1",
                    "cjjlfs": "合格制",
                    "kclb": "11",
                    "kctx": "思想政治"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425100131460_32220",
                    "jxbh": "3",
                    "kctxm": "03",
                    "xqcj": "86",
                    "xq": "1",
                    "ywmc": "Linear Algebra (B)",
                    "skjsxm": "2206192127-向圣权$数学学院$助理教授",
                    "skjszgh": "2206192127(00001)",
                    "xnd": "24-25",
                    "kch": "00131460",
                    "bkcjbh": "bkcj2025010002236517",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2024-251",
                    "kcmc": "线性代数 (B)",
                    "xndpx": "2024-25",
                    "xf": "4",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425100132511_41057",
                    "jxbh": "4",
                    "kctxm": "03",
                    "xqcj": "95.5",
                    "xq": "1",
                    "ywmc": "Advanced Mathematics A (no. 1)",
                    "skjsxm": "0006179125-束琳$数学学院$长聘副教授",
                    "skjszgh": "0006179125(00001)",
                    "xnd": "24-25",
                    "kch": "00132511",
                    "bkcjbh": "bkcj2024120002215883",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2024-251",
                    "kcmc": "高等数学A（一）",
                    "xndpx": "2024-25",
                    "xf": "5",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425100431141_11430",
                    "jxbh": "1",
                    "kctxm": "03",
                    "xqcj": "100",
                    "xq": "1",
                    "ywmc": "Mechanics",
                    "skjsxm": "0006167407-刘树新$物理学院$副教授",
                    "skjszgh": "0006167407(00004)",
                    "xnd": "24-25",
                    "kch": "00431141",
                    "bkcjbh": "bkcj2025010002260159",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2024-251",
                    "kcmc": "力学",
                    "xndpx": "2024-25",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425100431143_17114",
                    "jxbh": "1",
                    "kctxm": "03",
                    "xqcj": "EX",
                    "xq": "1",
                    "ywmc": "Electromagnetism",
                    "skjsxm": "",
                    "skjszgh": "",
                    "xnd": "24-25",
                    "kch": "00431143",
                    "bkcjbh": "bkcj2025010002238921",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2024-251",
                    "kcmc": "电磁学",
                    "xndpx": "2024-25",
                    "xf": "3",
                    "cjjlfs": "合格制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425100432001_11110",
                    "jxbh": "1",
                    "kctxm": "03",
                    "xqcj": "88.5",
                    "xq": "1",
                    "ywmc": "Fundamentals of Theoretical Physics I",
                    "skjsxm": "0006175231-孟策$物理学院$副教授",
                    "skjszgh": "0006175231(00004)",
                    "xnd": "24-25",
                    "kch": "00432001",
                    "bkcjbh": "bkcj2025010002256071",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2024-251",
                    "kcmc": "理论物理基础 I",
                    "xndpx": "2024-25",
                    "xf": "4",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425100434101_12262",
                    "jxbh": "1",
                    "kctxm": "04",
                    "xqcj": "合格",
                    "xq": "1",
                    "ywmc": "Distinguished Lecture Series in Physics: When Excellence Meets (I)",
                    "skjsxm": "1806163715-高原宁$物理学院$教授,0006178256-孙琰$物理学院$助理研究员",
                    "skjszgh": "1806163715(00004),0006178256(00004)",
                    "xnd": "24-25",
                    "kch": "00434101",
                    "bkcjbh": "bkcj2025010002296499",
                    "kclbmc": "任选",
                    "xndxqpx": "2024-251",
                    "kcmc": "物理卓越计划讲堂：名师面对面（一）",
                    "xndpx": "2024-25",
                    "xf": "1",
                    "cjjlfs": "合格制",
                    "kclb": "30",
                    "kctx": "专业任选"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425100437190_10030",
                    "jxbh": "1",
                    "kctxm": "03",
                    "xqcj": "95",
                    "xq": "1",
                    "ywmc": "General Physics Lab (2)",
                    "skjsxm": "0006179115-李智$物理学院$教授,1706184184-王伟$物理学院$工程师",
                    "skjszgh": "0006179115(00004),1706184184(00004)",
                    "xnd": "24-25",
                    "kch": "00437190",
                    "bkcjbh": "bkcj2025010002263609",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2024-251",
                    "kcmc": "普通物理实验（2）",
                    "xndpx": "2024-25",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425100437700_12174",
                    "jxbh": "1",
                    "kctxm": "04",
                    "xqcj": "合格",
                    "xq": "1",
                    "ywmc": "The Physics Application and Practice",
                    "skjsxm": "1606186110-荣新$物理学院$高级工程师,0006180094-廖慧敏$物理学院$副教授",
                    "skjszgh": "1606186110(00004),0006180094(00004)",
                    "xnd": "24-25",
                    "kch": "00437700",
                    "bkcjbh": "bkcj2025010002221916",
                    "kclbmc": "任选",
                    "xndxqpx": "2024-251",
                    "kcmc": "物理应用与实践",
                    "xndpx": "2024-25",
                    "xf": "1",
                    "cjjlfs": "合格制",
                    "kclb": "30",
                    "kctx": "专业任选"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425104031652113044",
                    "jxbh": "11",
                    "kctxm": "08",
                    "xqcj": "88",
                    "xq": "1",
                    "ywmc": "Ideology, morality and Law",
                    "skjsxm": "1906185159-钟启东$马克思学院$助理教授",
                    "skjszgh": "1906185159(00040)",
                    "xnd": "24-25",
                    "kch": "04031652",
                    "bkcjbh": "bkcj2025010002220432",
                    "kclbmc": "全校必修",
                    "xndxqpx": "2024-251",
                    "kcmc": "思想道德与法治",
                    "xndpx": "2024-25",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "11",
                    "kctx": "思想政治"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425104831410103140",
                    "jxbh": "10",
                    "kctxm": "02",
                    "xqcj": "98",
                    "xq": "1",
                    "ywmc": "Introduction to Computation (B)",
                    "skjsxm": "2006188105-李锭$计算机学院$助理教授",
                    "skjszgh": "2006188105(00101)",
                    "xnd": "24-25",
                    "kch": "04831410",
                    "bkcjbh": "bkcj2024120002200565",
                    "kclbmc": "全校必修",
                    "xndxqpx": "2024-251",
                    "kcmc": "计算概论（B）",
                    "xndpx": "2024-25",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "11",
                    "kctx": "理科生必修"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425160730020_68167",
                    "jxbh": "6",
                    "kctxm": "10",
                    "xqcj": "98",
                    "xq": "1",
                    "ywmc": "Military Theory",
                    "skjsxm": "0006165122-李纬华$武装部$助理研究员",
                    "skjszgh": "0006165122(00607)",
                    "xnd": "24-25",
                    "kch": "60730020",
                    "bkcjbh": "bkcj2025010002219951",
                    "kclbmc": "全校必修",
                    "xndxqpx": "2024-251",
                    "kcmc": "军事理论",
                    "xndpx": "2024-25",
                    "xf": "2",
                    "cjjlfs": "百分制",
                    "kclb": "11",
                    "kctx": "军事理论"
                },
                {
                    "xslb": "学籍生",
                    "zxjhbh": "BZ2425161130030_14844",
                    "jxbh": "1",
                    "kctxm": "08",
                    "xqcj": "合格",
                    "xq": "1",
                    "ywmc": "Social practice and service learning, Part I",
                    "skjsxm": "",
                    "skjszgh": "",
                    "xnd": "24-25",
                    "kch": "61130030",
                    "bkcjbh": "bkcj24-252025030002320040",
                    "kclbmc": "全校必修",
                    "xndxqpx": "2024-251",
                    "kcmc": "思想政治实践（上）",
                    "xndpx": "2024-25",
                    "xf": "1",
                    "cjjlfs": "合格制",
                    "kclb": "11",
                    "kctx": "思想政治"
                },
                {
                    "xslb": "旁听生",
                    "zxjhbh": "BZ2324200431156_32609",
                    "jxbh": "3",
                    "kctxm": "03",
                    "xqcj": "94",
                    "xq": "2",
                    "ywmc": "Optics",
                    "skjsxm": "0006166482-李焱$物理学院$教授",
                    "skjszgh": "0006166482(00004)",
                    "xnd": "23-24",
                    "kch": "00431156",
                    "bkcjbh": "bkcj2024090002197862",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2023-242",
                    "kcmc": "光学",
                    "xndpx": "2023-24",
                    "xf": "4",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "旁听生",
                    "zxjhbh": "BZ2324200431165_21934",
                    "jxbh": "2",
                    "kctxm": "03",
                    "xqcj": "97",
                    "xq": "2",
                    "ywmc": "Modern Physics",
                    "skjsxm": "0006171250-华辉$物理学院$教授",
                    "skjszgh": "0006171250(00004)",
                    "xnd": "23-24",
                    "kch": "00431165",
                    "bkcjbh": "bkcj2024090002197864",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2023-242",
                    "kcmc": "近代物理",
                    "xndpx": "2023-24",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "旁听生",
                    "zxjhbh": "BZ2324200437180_18698",
                    "jxbh": "1",
                    "kctxm": "03",
                    "xqcj": "93",
                    "xq": "2",
                    "ywmc": "General Physics Lab (1)",
                    "skjsxm": "0006179115-李智$物理学院$教授,1706184184-王伟$物理学院$工程师",
                    "skjszgh": "0006179115(00004),1706184184(00004)",
                    "xnd": "23-24",
                    "kch": "00437180",
                    "bkcjbh": "bkcj2024090002197863",
                    "kclbmc": "专业必修",
                    "xndxqpx": "2023-242",
                    "kcmc": "普通物理实验（1）",
                    "xndpx": "2023-24",
                    "xf": "3",
                    "cjjlfs": "百分制",
                    "kclb": "13",
                    "kctx": "专业必修"
                },
                {
                    "xslb": "旁听生",
                    "zxjhbh": "BZ2324200437701_14816",
                    "jxbh": "1",
                    "kctxm": "04",
                    "xqcj": "合格",
                    "xq": "2",
                    "ywmc": "Physical Comprehensive Quality Development Course",
                    "skjsxm": "1106175319-曹庆宏$物理学院$教授",
                    "skjszgh": "1106175319(00004)",
                    "xnd": "23-24",
                    "kch": "00437701",
                    "bkcjbh": "bkcj2024090002197861",
                    "kclbmc": "任选",
                    "xndxqpx": "2023-242",
                    "kcmc": "卓越综合素质拓展",
                    "xndpx": "2023-24",
                    "xf": "2",
                    "cjjlfs": "合格制",
                    "kclb": "30",
                    "kctx": "专业任选"
                }
            ],
            "zjlcjxx": [
                []
            ],
            "bylwcjxx": [],
            "gpaHM": {
                "txkxf": "5.0",
                "rxxf": "7.0",
                "bxxxbjgms": "0",
                "zxfgpa": "77.0",
                "gpaflag": "y",
                "gpa": "3.892",
                "bjgzxf": "0.0",
                "zxf": "93.0",
                "xkms": "36",
                "jdsum": "299.68",
                "bxxf": "81.0",
                "xxxf": "93",
                "jd": "3.892",
                "xzxf": "0.0",
                "bjgms": "0",
                "jlzxf": "0.0"
            },
            "gpa": {
                "xxxf": "93",
                "gpa": "3.892"
            }
        },
        "gpa": {
            "success": true,
            "scFormats": [
                {
                    "colId": "xndxq",
                    "colName": "学年度学期",
                    "colOrder": "0"
                },
                {
                    "colId": "gpa",
                    "colName": "绩点",
                    "colOrder": "1"
                }
            ],
            "data": [
                {
                    "gpa": "3.925",
                    "grade": "2024",
                    "xndxq": "25-26-1"
                },
                {
                    "gpa": "3.883",
                    "grade": "2024",
                    "xndxq": "24-25-2"
                },
                {
                    "gpa": "3.863",
                    "grade": "2024",
                    "xndxq": "24-25-1"
                }
            ]
        }
    },
    "message": "success",
    "success": true,
    "timestamp": 1781716157
}


# 数据结构解读

## post
eg: {
    "pid": 8139378,
    "text": "zhf思修\n哪来的嘉豪啊😓\n一直追着台上pre的同学要个立场是何意味，给了台阶都不下说是。\n你都表明自己观点了那追着人家个只能打圆场的问有什么意义吗？那我只能归结于你想要炫耀自己的与众不同的观点了",
    "type": "text",
    "timestamp": 1775733384,
    "hidden": 0,
    "reply": 104,
    "likenum": 67,
    "extra": 0,
    "anonymous": 1,
    "hot": 1775733384,
    "tag": null,
    "protected": 0,
    "is_top": 0,
    "label": 191,
    "status": 0,
    "is_comment": 1,
    "tags_ids": "191",
    "auto_tags_ids": "191",
    "mgr_tags_ids": "",
    "media_ids": "",
    "fold": 0,
    "kind": 0,
    "reward_cost": 0,
    "reward_state": 0,
    "identity_show": 0,
    "identity_type": "",
    "exclusive_id_id": 0,
    "mention": "",
    "mailbox": 0,
    "image_size": "",
    "has_reward_good": 0,
    "tags_info": [],
    "tags_list": [],
    "exclusive_id_info": [],
    "identity_info": [],
    "is_god_hole": 1,
    "is_protect": 0,
    "tread_num": 0,
    "praise_num": 0,
    "praise_num_show": 0,
    "fold_num": 0,
    "is_sdss": 0,
    "is_follow": 0,
    "attention_info": [],
    "is_praise": 0,
    "is_tread": 0,
    "islz": 0,
    "user_fold": 0,
    "user_config_fold": 1,
    "is_fold": 0,
    "cannot_reply": 0
}


要点：
+ "hot" 与 "timestamp" 相同
+ "label" 其实是tag的语义，"tag" 字段似乎无用
+ "is_god_hole"表示热度高，判断标准尚不明确
+ "auto_tags_ids"是后台自动打的tag
+ "tags_info"，"tags_list"，"exclusive_id_info"，"identity_info"，"attention_info" 几个字段空时为[]，非空时为{xxx:xxx}的对象，解析时需要注意
+ "likenum"实为关注数，"is_follow"是用户是否已关注，"praise_num"才是点赞，"praise_num_show"总是与"praise_num"相同
+ "kind"是树洞类型，0为普通树洞，1为悬赏树洞，与reward相关的字段联系

## comment
eg: {
    "cid": 37458733,
    "pid": 8139378,
    "text": "666还在群里追着杀😓",
    "timestamp": 1775733723,
    "hidden": 0,
    "anonymous": 1,
    "tag": null,
    "comment_id": null,
    "name_tag": "洞主",
    "media_ids": "16204",
    "reward_good": 0,
    "identity_show": 0,
    "identity_type": "",
    "exclusive_id_id": 0,
    "mention": "",
    "quote": [],
    "exclusive_id_info": [],
    "identity_info": [],
    "is_author": 0,
    "is_lz": 1
}

要点：
+ "is_lz"是否楼主
+ "is_author"表示当前用户是否是该评论的作者
+ "quote"，"exclusive_id_info"，"identity_info"，字段为空时为[]，非空时为{xxx:xxx}的对象，解析时需要注意

总之里面有很多旧版本的遗产，语义不一致的遗留问题，务必注意

更多的示例放在doc/comments_examples.json,doc/post_examples.json中

# 认证
把cookies都带上，headers中有uuid,x-xsrf-token,authorization