const { createProxyMiddleware } = require('http-proxy-middleware');

module.exports = function(app) {
  // 代理 API 请求到后端
  app.use(
    '/api',
    createProxyMiddleware({
      target: 'http://localhost:8080',
      changeOrigin: true,
      logLevel: 'debug',
    })
  );

  // 代理认证请求到后端
  app.use(
    '/auth',
    createProxyMiddleware({
      target: 'http://localhost:8080',
      changeOrigin: true,
      logLevel: 'debug',
    })
  );
};
