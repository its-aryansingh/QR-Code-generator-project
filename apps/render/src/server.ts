import Fastify from 'fastify';
import { processRender, RenderRequest } from './render.js';

const server = Fastify({
  logger: {
    level: process.env.LOG_LEVEL || 'info',
  },
});

server.get('/healthz', async () => {
  return { status: 'ok', uptime: process.uptime() };
});

server.post('/render', async (request, reply) => {
  const body = request.body as Partial<RenderRequest>;

  if (!body || typeof body !== 'object') {
    return reply.status(400).send({
      type: 'https://qrit.io/errors/bad-request',
      title: 'Bad Request',
      status: 400,
      detail: 'Request body must be a JSON object',
    });
  }

  if (!body.payload || typeof body.payload !== 'string') {
    return reply.status(400).send({
      type: 'https://qrit.io/errors/validation',
      title: 'Validation Error',
      status: 400,
      detail: 'payload is required and must be a string',
    });
  }

  const format = body.format || 'svg';
  if (!['svg', 'png', 'pdf'].includes(format)) {
    return reply.status(400).send({
      type: 'https://qrit.io/errors/validation',
      title: 'Validation Error',
      status: 400,
      detail: "format must be 'svg', 'png', or 'pdf'",
    });
  }

  const sizePx = body.sizePx && typeof body.sizePx === 'number' && body.sizePx > 0
    ? Math.min(4096, Math.max(64, body.sizePx))
    : 512;

  try {
    const result = await processRender({
      payload: body.payload,
      design: body.design,
      format,
      sizePx,
    });

    reply.header('Content-Type', result.contentType);
    reply.header('X-QR-Version', result.version.toString());
    reply.header('X-QR-ECC', result.ecc);
    reply.header('X-QR-Warnings', JSON.stringify(result.warnings));

    return reply.send(result.data);
  } catch (err: any) {
    request.log.error(err, 'Failed to process render request');
    return reply.status(500).send({
      type: 'https://qrit.io/errors/internal',
      title: 'Render Error',
      status: 500,
      detail: err?.message || 'Failed to render QR code',
    });
  }
});

const start = async () => {
  const port = parseInt(process.env.PORT || '3001', 10);
  const host = process.env.HOST || '0.0.0.0';

  try {
    await server.listen({ port, host });
    server.log.info(`Render service listening on ${host}:${port}`);
  } catch (err) {
    server.log.error(err);
    process.exit(1);
  }
};

start();
