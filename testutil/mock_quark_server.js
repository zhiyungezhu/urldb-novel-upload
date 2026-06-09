/**
 * 夸克网盘 Mock API 服务器
 * 用于本地模拟上传、分享等全部流程，无需真实夸克账号
 *
 * 启动: node testutil/mock_quark_server.js
 * 端口: 9999
 *
 * 模拟的接口:
 *   POST /1/clouddrive/file          → 预上传（秒传检测 + 获取上传URL）
 *   PUT  /upload/:fileId             → 文件内容上传
 *   GET  /1/clouddrive/task          → 任务状态查询
 *   GET  /1/clouddrive/file/sort     → 文件列表
 *   POST /1/clouddrive/share         → 创建分享
 *   POST /1/clouddrive/share/password → 获取分享链接
 */

const http = require("http");
const url = require("url");
const crypto = require("crypto");
const fs = require("fs");
const path = require("path");

const PORT = 9999;
const HOST = "0.0.0.0";

// 模拟网盘文件系统（内存中）
const virtualDrive = {
  files: new Map(), // fid -> { fid, file_name, file_size, sha1, created_at }
  shares: new Map(), // shareID -> { share_id, share_url, fid_list, title, code }
  tasks: new Map(), // taskID -> { status, data }
  nextFid: 1000,
  nextTaskId: 2000,
  nextShareId: 3000,
};

function generateId(prefix) {
  return `${prefix}_${Date.now()}_${crypto.randomBytes(8).toString("hex")}`;
}

function log(method, pathname, body) {
  const ts = new Date().toISOString().substring(11, 23);
  const bodyStr = body
    ? JSON.stringify(body).substring(0, 80)
    : "(no body)";
  console.log(`[${ts}] ${method} ${pathname} → ${bodyStr}`);
}

/**
 * 预上传 / 秒传检测
 * POST /1/clouddrive/file
 * Body: { pdir_fid, file_name, file_size, sha1 }
 */
function handlePreUpload(body, req) {
  const { pdir_fid, file_name, file_size, sha1 } = body;

  console.log(`  📁 预上传: ${file_name} (${file_size} bytes, SHA1: ${sha1})`);

  // 秒传检测：查找相同 SHA1 的文件
  for (const [fid, file] of virtualDrive.files) {
    if (file.sha1 === sha1) {
      console.log(`  ⚡ 秒传命中！已存在文件 fid=${fid}`);
      return {
        status: 200,
        code: 0,
        message: "秒传成功",
        data: {
          fid: fid,
          finish: true,
        },
      };
    }
  }

  // 正常上传：返回上传 URL 和任务 ID
  const fid = `${virtualDrive.nextFid++}`;
  const taskID = `${virtualDrive.nextTaskId++}`;
  const uploadToken = crypto.randomBytes(16).toString("hex");

  // 创建上传任务
  virtualDrive.tasks.set(taskID, {
    status: 1, // 1 = 上传中, 2 = 完成
    fid: fid,
    file_name: file_name,
    file_size: file_size,
    sha1: sha1,
    created_at: new Date().toISOString(),
  });

  const uploadURL = `http://${req.headers.host}/upload/${uploadToken}?fid=${fid}&task_id=${taskID}`;

  console.log(`  ✅ 返回上传URL: ${uploadURL} (taskID=${taskID}, fid=${fid})`);

  return {
    status: 200,
    code: 0,
    message: "",
    data: {
      task_id: taskID,
      upload_url: uploadURL,
      fid: null,
      finish: false,
    },
  };
}

/**
 * 文件内容上传
 * PUT /upload/:token?fid=xxx&task_id=xxx
 */
function handleFileUpload(req, res, params) {
  const { fid, task_id } = params;

  console.log(`  📤 文件上传中... fid=${fid}, taskID=${task_id}`);

  const chunks = [];
  req.on("data", (chunk) => chunks.push(chunk));
  req.on("end", () => {
    const content = Buffer.concat(chunks);
    const task = virtualDrive.tasks.get(task_id);

    if (!task) {
      res.writeHead(404, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ status: 500, message: "任务不存在" }));
      return;
    }

    // 保存文件到虚拟网盘
    virtualDrive.files.set(fid, {
      fid: fid,
      file_name: task.file_name,
      file_size: content.length,
      sha1: task.sha1,
      content: content,
      pdir_fid: "0",
      created_at: new Date().toISOString(),
    });

    // 完成任务
    task.status = 2; // 完成
    task.data = {
      fid: fid,
      file_name: task.file_name,
    };

    console.log(
      `  ✅ 文件上传完成! fid=${fid}, 文件名=${task.file_name}, 大小=${content.length}`
    );

    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ success: true }));
  });
}

/**
 * 任务状态查询
 * GET /1/clouddrive/task?task_id=xxx
 */
function handleTaskQuery(params) {
  const taskID = params.task_id;
  const task = virtualDrive.tasks.get(taskID);

  if (!task) {
    return { status: 404, code: 1, message: "任务不存在" };
  }

  // 模拟异步：第一次查询返回 pending，第二次返回完成
  // 实际夸克API: status 0=处理中, 1=处理中, 2=完成
  if (task.status === 1 && !task._queried) {
    task._queried = true;
    console.log(`  ⏳ 任务 ${taskID}: 处理中...`);
    return {
      status: 200,
      code: 0,
      data: {
        task_id: taskID,
        status: 0, // 处理中
        share_id: null,
      },
    };
  }

  if (task.status === 2 || task._queried) {
    // 分享任务返回 share_id
    const shareID = task.share_id
      ? `share_${virtualDrive.nextShareId++}`
      : null;
    if (!task.share_id) {
      task.share_id = shareID;
    }

    console.log(
      `  ✅ 任务 ${taskID}: 完成! ${task.share_id ? `shareID=${shareID}` : `fid=${task.fid}`}`
    );

    return {
      status: 200,
      code: 0,
      data: {
        task_id: taskID,
        status: 2, // 完成
        share_id: task.share_id || null,
        fid: task.fid,
        file_name: task.file_name,
      },
    };
  }

  return {
    status: 200,
    code: 0,
    data: {
      task_id: taskID,
      status: 0,
      share_id: null,
    },
  };
}

/**
 * 文件列表
 * GET /1/clouddrive/file/sort
 */
function handleFileList(params) {
  const files = [];
  for (const [, file] of virtualDrive.files) {
    files.push({
      fid: file.fid,
      file_name: file.file_name,
      file_size: file.file_size,
      pdir_fid: file.pdir_fid || "0",
      created_at: file.created_at,
      obj_category: "file",
    });
  }

  console.log(`  📋 文件列表: ${files.length} 个文件`);

  return {
    status: 200,
    code: 0,
    data: {
      list: files,
      count: files.length,
    },
  };
}

/**
 * 创建分享
 * POST /1/clouddrive/share
 * Body: { fid_list, title, url_type, expired_type }
 */
function handleCreateShare(body) {
  const { fid_list, title } = body;

  console.log(`  🔗 创建分享: "${title}", 文件: [${fid_list}]`);

  const taskID = `${virtualDrive.nextTaskId++}`;
  const shareID = `share_${virtualDrive.nextShareId++}`;

  // 创建分享任务
  virtualDrive.tasks.set(taskID, {
    status: 1,
    share_id: shareID,
    fid_list: fid_list,
    title: title,
    created_at: new Date().toISOString(),
    _type: "share",
  });

  // 预创建分享记录
  virtualDrive.shares.set(shareID, {
    share_id: shareID,
    share_url: `https://pan.mock.quark.cn/s/${crypto.randomBytes(5).toString("hex")}`,
    fid_list: fid_list,
    title: title,
    code: crypto.randomBytes(2).toString("hex"),
    first_file: { fid: fid_list[0] },
    status: "normal",
  });

  console.log(`  ✅ 分享任务创建: taskID=${taskID}, shareID=${shareID}`);

  return {
    status: 200,
    code: 0,
    data: {
      task_id: taskID,
      share_id: shareID,
    },
  };
}

/**
 * 获取分享密码
 * POST /1/clouddrive/share/password
 */
function handleSharePassword(body) {
  const shareID = body.share_id;
  const share = virtualDrive.shares.get(shareID);

  if (!share) {
    return { status: 404, code: 1, message: "分享不存在" };
  }

  console.log(`  🔑 获取分享链接: shareID=${shareID} → ${share.share_url}`);

  return {
    status: 200,
    code: 0,
    data: {
      share_id: share.share_id,
      share_url: share.share_url,
      share_title: share.title,
      code: share.code,
      first_file: {
        fid: share.first_file.fid,
      },
    },
  };
}

// ==================== HTTP Server ====================

const server = http.createServer((req, res) => {
  // CORS
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader(
    "Access-Control-Allow-Methods",
    "GET, POST, PUT, DELETE, OPTIONS"
  );
  res.setHeader("Access-Control-Allow-Headers", "Content-Type, Cookie");

  if (req.method === "OPTIONS") {
    res.writeHead(200);
    res.end();
    return;
  }

  const parsed = url.parse(req.url, true);
  const pathname = parsed.pathname;
  const method = req.method;

  // 文件上传 (PUT /upload/...) 需要流式处理，不预先收集 body
  if (method === "PUT" && pathname.startsWith("/upload/")) {
    log(method, pathname, "(binary stream)");
    const queryParams = {};
    if (parsed.query) Object.assign(queryParams, parsed.query);
    handleFileUpload(req, res, queryParams);
    return;
  }

  // 其他请求：收集 body
  let bodyRaw = [];
  req.on("data", (chunk) => bodyRaw.push(chunk));
  req.on("end", () => {
    let body = {};
    try {
      body = Buffer.concat(bodyRaw).toString();
      if (body) body = JSON.parse(body);
      else body = {};
    } catch {
      body = {};
    }

    log(method, pathname, body);
    let result;

    try {
      // 预上传 (POST /1/clouddrive/file)
      if (method === "POST" && pathname === "/1/clouddrive/file") {
        result = handlePreUpload(body, req);
      }
      // 任务查询
      else if (pathname.includes("/task")) {
        result = handleTaskQuery(parsed.query);
      }
      // 文件列表
      else if (pathname.includes("/file/sort")) {
        result = handleFileList(parsed.query);
      }
      // 创建分享
      else if (method === "POST" && pathname === "/1/clouddrive/share") {
        result = handleCreateShare(body);
      }
      // 分享密码
      else if (pathname.includes("/share/password")) {
        result = handleSharePassword(body);
      }
      // 未知路由
      else {
        console.log(`  ❓ 未知路由: ${method} ${pathname}`);
        result = { status: 404, message: `Mock: 路由未实现 ${pathname}` };
      }
    } catch (err) {
      console.error(`  ❌ 错误:`, err.message);
      result = { status: 500, message: err.message };
    }

    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify(result));
  });
});

server.listen(PORT, HOST, () => {
  console.log("");
  console.log("╔══════════════════════════════════════════════╗");
  console.log("║   🧪 夸克网盘 Mock API 服务器已启动           ║");
  console.log(`║   地址: http://localhost:${PORT}               ║`);
  console.log("║                                              ║");
  console.log("║  使用方式:                                    ║");
  console.log(`║   1. 设置环境变量:                             ║`);
  console.log(`║      \$env:QUARK_API_BASE_URL="http://localhost:${PORT}"  ║`);
  console.log("║   2. 启动 Go 程序                              ║");
  console.log("║   3. 把文件放入 ./pending_upload/ 目录          ║");
  console.log("║                                              ║");
  console.log("║  模拟的 API:                                  ║");
  console.log("║    POST /1/clouddrive/file  → 预上传          ║");
  console.log("║    PUT  /upload/:token       → 文件上传       ║");
  console.log("║    GET  /1/clouddrive/task   → 任务查询       ║");
  console.log("║    GET  /1/clouddrive/file/sort → 文件列表    ║");
  console.log("║    POST /1/clouddrive/share  → 创建分享       ║");
  console.log("║    POST /1/clouddrive/share/password → 分享   ║");
  console.log("╚══════════════════════════════════════════════╝");
  console.log("");
});
