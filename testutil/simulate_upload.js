/**
 * 本地上传流程模拟测试
 * 纯 Node.js，无需 Go/Docker，直接验证上传→分享全流程
 *
 * 前提: 先启动 mock_quark_server.js
 * 使用: node testutil/simulate_upload.js [文件路径]
 *       不传参数则自动创建一个测试文件
 */

const http = require("http");
const https = require("https");
const fs = require("fs");
const crypto = require("crypto");
const path = require("path");
const { URL } = require("url");

const BASE = process.env.QUARK_API_BASE_URL || "http://localhost:9999";
const TEST_FILE = process.argv[2] || path.join(__dirname, "test_upload_file.txt");

function request(method, urlPath, body) {
  return new Promise((resolve, reject) => {
    const url = new URL(urlPath, BASE);
    const client = url.protocol === "https:" ? https : http;

    const options = {
      hostname: url.hostname,
      port: url.port,
      path: url.pathname + url.search,
      method,
      headers: { "Content-Type": "application/json" },
    };

    const req = client.request(options, (res) => {
      let data = "";
      res.on("data", (chunk) => (data += chunk));
      res.on("end", () => {
        try {
          resolve(JSON.parse(data));
        } catch {
          resolve(data);
        }
      });
    });

    req.on("error", reject);
    if (body) req.write(JSON.stringify(body));
    req.end();
  });
}

function fileUpload(uploadUrl, filePath) {
  return new Promise((resolve, reject) => {
    const url = new URL(uploadUrl);
    const client = url.protocol === "https:" ? https : http;
    const fileSize = fs.statSync(filePath).size;

    const options = {
      hostname: url.hostname,
      port: url.port,
      path: url.pathname + url.search,
      method: "PUT",
      headers: {
        "Content-Type": "application/octet-stream",
        "Content-Length": fileSize,
      },
    };

    const req = client.request(options, (res) => {
      let data = "";
      res.on("data", (chunk) => (data += chunk));
      res.on("end", () => {
        try {
          resolve(JSON.parse(data));
        } catch {
          resolve(data);
        }
      });
    });

    req.on("error", reject);
    fs.createReadStream(filePath).pipe(req);
  });
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

async function main() {
  console.log("╔══════════════════════════════════════════╗");
  console.log("║   🧪 夸克上传流程本地模拟测试              ║");
  console.log(`║   Mock 服务器: ${BASE}                  ║`);
  console.log("╚══════════════════════════════════════════╝\n");

  // 1. 准备测试文件
  if (!fs.existsSync(TEST_FILE)) {
    const content = `测试文件 - 创建时间: ${new Date().toISOString()}\n${crypto.randomBytes(512).toString("hex")}`;
    fs.writeFileSync(TEST_FILE, content);
    console.log(`[1/7] 📄 创建测试文件: ${TEST_FILE} (${fs.statSync(TEST_FILE).size} bytes)`);
  } else {
    console.log(`[1/7] 📄 使用已有文件: ${TEST_FILE} (${fs.statSync(TEST_FILE).size} bytes)`);
  }

  const fileName = path.basename(TEST_FILE);
  const fileSize = fs.statSync(TEST_FILE).size;

  // 2. 计算 SHA1
  const fileBuffer = fs.readFileSync(TEST_FILE);
  const sha1 = crypto.createHash("sha1").update(fileBuffer).digest("hex");
  console.log(`[2/7] 🔑 SHA1: ${sha1}`);

  // 3. 预上传
  console.log(`[3/7] 📤 预上传请求...`);
  const preUploadResult = await request("POST", "/1/clouddrive/file", {
    pdir_fid: "0",
    file_name: fileName,
    file_size: fileSize,
    sha1: sha1,
  });

  if (preUploadResult.data?.finish) {
    console.log(`  ⚡ 秒传成功! fid=${preUploadResult.data.fid}`);
  } else {
    const uploadUrl = preUploadResult.data.upload_url;
    const taskId = preUploadResult.data.task_id;
    console.log(`  ✅ 获取上传URL: ${uploadUrl}`);
    console.log(`  📋 任务ID: ${taskId}`);

    // 4. 上传文件内容
    console.log(`[4/7] 📤 上传文件内容 (${fileSize} bytes)...`);
    const uploadResult = await fileUpload(uploadUrl, TEST_FILE);
    console.log(`  ✅ 上传完成:`, JSON.stringify(uploadResult));

    // 5. 等待上传任务完成
    console.log(`[5/7] ⏳ 等待上传任务完成...`);
    await sleep(1500);
    const taskResult = await request("GET", `/1/clouddrive/task?task_id=${taskId}`);
    console.log(`  ✅ 任务完成: status=${taskResult.data.status}`);
  }

  // 6. 获取文件列表，找到刚上传的 fid
  console.log(`[6/7] 📋 获取文件列表...`);
  const fileList = await request("GET", "/1/clouddrive/file/sort?pdir_fid=0");
  const uploadedFile = fileList.data?.list?.find((f) => f.file_name === fileName);
  if (!uploadedFile) {
    console.log("  ❌ 未找到上传的文件！");
    process.exit(1);
  }
  console.log(`  ✅ 找到文件: fid=${uploadedFile.fid}, name=${uploadedFile.file_name}`);

  // 7. 创建分享并获取链接
  console.log(`[7/7] 🔗 创建分享...`);
  const shareResult = await request("POST", "/1/clouddrive/share", {
    fid_list: [uploadedFile.fid],
    title: fileName,
    url_type: 1,
    expired_type: 1,
  });

  // 等待分享任务完成
  await sleep(1000);
  const shareTaskResult = await request(
    "GET",
    `/1/clouddrive/task?task_id=${shareResult.data.task_id}`
  );

  // 取直接返回的 share_id（share 创建时就已分配）
  const shareId = shareResult.data.share_id || shareTaskResult.data.share_id;
  if (!shareId) {
    console.log("  ❌ 未获取到分享ID");
    process.exit(1);
  }
  const passwordResult = await request("POST", "/1/clouddrive/share/password", {
    share_id: shareId,
  });

  console.log("\n╔══════════════════════════════════════════╗");
  console.log("║   ✅ 全部流程完成！                       ║");
  console.log("╠══════════════════════════════════════════╣");
  console.log(`║   文件名:     ${fileName}`);
  console.log(`║   文件大小:   ${fileSize} bytes`);
  console.log(`║   SHA1:       ${sha1.substring(0, 16)}...`);
  console.log(`║   FID:        ${uploadedFile.fid}`);
  console.log(`║   分享链接:   ${passwordResult.data.share_url}`);
  console.log(`║   提取码:     ${passwordResult.data.code}`);
  console.log("╚══════════════════════════════════════════╝\n");

  // 清理测试文件（如果是自动创建的）
  if (!process.argv[2]) {
    fs.unlinkSync(TEST_FILE);
    console.log("🧹 已清理测试文件\n");
  }
}

main().catch((err) => {
  console.error("❌ 测试失败:", err.message);
  process.exit(1);
});
