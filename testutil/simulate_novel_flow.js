/**
 * 完整小说上传流程模拟
 * 演示：建文件夹 → 批量上传 → 分享文件夹
 *
 * 前置条件：json 服务器在 localhost:9999 运行
 */

const http = require('http');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const MOCK_URL = 'http://localhost:9999';
const COOKIE = '__puus=mock_token; __quark_pus=mock_pus';
const PARENT_FID = '0';         // 根目录（用户手动创建小说文件夹后替换）
const NOVEL_NAME = '斗破苍穹';

async function main() {
  console.log('╔══════════════════════════════════════════════════════════╗');
  console.log('║   完整小说上传流程模拟                                   ║');
  console.log('║   ① 在夸克创建小说文件夹                                 ║');
  console.log('║   ② 批量上传小说文件                                     ║');
  console.log('║   ③ 分享文件夹获取永久链接                               ║');
  console.log('╚══════════════════════════════════════════════════════════╝\n');

  // ======================== 第1步：查询用户信息（验证Cookie） ========================
  console.log('【第0步】验证 Cookie - 查询用户信息');
  // 注：getUserInfo 调用的是 pan.quark.cn/account/info
  // Mock 服务器的 /account/info 端点在 /1/clouddrive/member 之下

  // ======================== 第1步：创建小说文件夹 ========================
  console.log(`\n【第1步】在夸克创建小说文件夹: ${NOVEL_NAME}`);
  console.log(`  POST /1/clouddrive/file`);
  console.log(`  Body: { pdir_fid: "${PARENT_FID}", file_name: "斗破苍穹", dir: 1 }`);

  const mkdirBody = JSON.stringify({
    pdir_fid: PARENT_FID,
    file_name: NOVEL_NAME,
    dir: 1
  });

  const mkdirRes = await fetch(`${MOCK_URL}/1/clouddrive/file`, {
    method: 'POST',
    headers: { 'Cookie': COOKIE, 'Content-Type': 'application/json' },
    body: mkdirBody
  });
  const mkdirData = await mkdirRes.json();
  const novelFolderFid = mkdirData.data?.fid || 'mock_folder_fid_001';
  console.log(`  ✅ 文件夹创建成功 → fid: ${novelFolderFid}`);
  console.log(`  （注：真实场景中，你先在浏览器手动创建"小说"文件夹，取它的 fid 作为 NOVEL_PARENT_FID）\n`);

  // ======================== 第2步：批量上传小说文件 ========================
  console.log('【第2步】批量上传小说文件到该文件夹');
  const testFiles = [
    { name: '斗破苍穹.epub',   size: '2.1 MB' },
    { name: '第1章_陨落的天才.txt', size: '35 KB' },
    { name: '第2章_斗者_斗师.txt',  size: '32 KB' },
    { name: '第3章_客从何处来.txt',  size: '38 KB' },
    { name: 'cover.jpg',       size: '156 KB' },
  ];

  const uploadedFids = [];
  for (const file of testFiles) {
    // 2a. 计算 SHA1（模拟）
    const sha1 = crypto.createHash('sha1').update(file.name).digest('hex');
    console.log(`  📄 ${file.name} (${file.size})`);

    // 2b. 预上传
    const preUploadBody = JSON.stringify({
      pdir_fid: novelFolderFid,
      file_name: file.name,
      file_size: parseInt(file.size) * 1024 || 1024,
      sha1: sha1
    });

    const preRes = await fetch(`${MOCK_URL}/1/clouddrive/file`, {
      method: 'POST',
      headers: { 'Cookie': COOKIE, 'Content-Type': 'application/json' },
      body: preUploadBody
    });
    const preData = await preRes.json();

    // 2c. 模拟 PUT 上传
    const uploadUrl = preData.data?.upload_url || `${MOCK_URL}/upload/mock_token_${Date.now()}`;
    const putRes = await fetch(uploadUrl, { method: 'PUT', body: 'mock content' });
    const putData = await putRes.json();

    // 2d. 轮询等待完成
    const taskId = preData.data?.task_id || '2000';
    for (let i = 0; i < 3; i++) {
      const taskRes = await fetch(`${MOCK_URL}/1/clouddrive/task?task_id=${taskId}`, {
        headers: { 'Cookie': COOKIE }
      });
      const taskData = await taskRes.json();
      if (taskData.data?.status === 2) break;
      await new Promise(r => setTimeout(r, 500));
    }

    const fid = preData.data?.fid || `fid_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`;
    uploadedFids.push(fid);
    console.log(`  ✅ 上传完成 → fid: ${fid}`);
  }

  console.log(`\n  📊 全部 ${uploadedFids.length}/${testFiles.length} 个文件上传成功\n`);

  // ======================== 第3步：分享文件夹 ========================
  console.log('【第3步】分享文件夹获取永久链接');

  // 3a. 获取文件夹下所有文件 fid（GET file/sort）
  console.log(`  GET /1/clouddrive/file/sort?pdir_fid=${novelFolderFid}`);
  const sortRes = await fetch(`${MOCK_URL}/1/clouddrive/file/sort?pdir_fid=${novelFolderFid}`, {
    headers: { 'Cookie': COOKIE }
  });
  const sortData = await sortRes.json();
  console.log(`  ✅ 获取到 ${uploadedFids.length} 个文件 fid`);

  // 3b. 创建分享
  console.log(`  POST /1/clouddrive/share`);
  const shareBody = JSON.stringify({
    fid_list: uploadedFids,
    title: NOVEL_NAME,
    url_type: 1,
    expired_type: 1   // 永久
  });
  const shareRes = await fetch(`${MOCK_URL}/1/clouddrive/share`, {
    method: 'POST',
    headers: { 'Cookie': COOKIE, 'Content-Type': 'application/json' },
    body: shareBody
  });
  const shareData = await shareRes.json();
  const shareTaskID = shareData.data?.task_id || 'mock_share_task';
  console.log(`  ✅ 分享创建中 → task_id: ${shareTaskID}`);

  // 3c. 等待分享完成
  await new Promise(r => setTimeout(r, 1000));
  let shareID;
  for (let i = 0; i < 5; i++) {
    const taskRes = await fetch(`${MOCK_URL}/1/clouddrive/task?task_id=${shareTaskID}`, {
      headers: { 'Cookie': COOKIE }
    });
    const taskData = await taskRes.json();
    shareID = taskData.data?.share_id;
    if (taskData.data?.status === 2 && shareID) break;
    await new Promise(r => setTimeout(r, 1000));
  }

  // 3d. 获取分享链接和密码
  const pwdRes = await fetch(`${MOCK_URL}/1/clouddrive/share/password`, {
    method: 'POST',
    headers: { 'Cookie': COOKIE, 'Content-Type': 'application/json' },
    body: JSON.stringify({ share_id: shareID || 'mock_share_001' })
  });
  const pwdData = await pwdRes.json();

  const shareUrl = pwdData.data?.share_url || `https://pan.mock.quark.cn/s/${Math.random().toString(36).slice(2, 12)}`;
  const shareCode = pwdData.data?.code || 'b57c';

  console.log(`  ✅ 分享完成！\n`);

  // ======================== 最终输出 ========================
  console.log('╔══════════════════════════════════════════════════════════╗');
  console.log('║   ✅ 小说上传全流程模拟完成！                           ║');
  console.log('╠══════════════════════════════════════════════════════════╣');
  console.log(`║   小说:        ${NOVEL_NAME}`);
  console.log(`║   文件数:      ${uploadedFids.length} 个`);
  console.log(`║   分享链接:    ${shareUrl}`);
  console.log(`║   提取码:      ${shareCode}`);
  console.log(`║   夸克文件夹:  已保存到夸克 → "${NOVEL_NAME}"`);
  console.log('╠══════════════════════════════════════════════════════════╣');
  console.log('║   技术细节：                                            ║');
  console.log(`║   建文件夹:    POST /1/clouddrive/file (dir=1)`);
  console.log(`║   预上传:      POST /1/clouddrive/file (sha1+size)`);
  console.log(`║   文件上传:    PUT {upload_url}`);
  console.log(`║   轮询:        GET /1/clouddrive/task`);
  console.log(`║   分享:        POST /1/clouddrive/share`);
  console.log(`║   获取密码:    POST /1/clouddrive/share/password`);
  console.log('╚══════════════════════════════════════════════════════════╝');

  console.log('\n📋 存储到数据库的 resource 记录：');
  console.log(JSON.stringify({
    title: `[小说] ${NOVEL_NAME} (${uploadedFids.length}文件)`,
    url: shareUrl,
    save_url: shareUrl,
    pan_id: 1,
    is_valid: true,
    is_public: true,
    ck_id: 1,
    fid: novelFolderFid,
  }, null, 2));

  console.log('\n⏰ 12小时后自动删除本地文件夹: D:/output/斗破苍穹/');
}

main().catch(console.error);
