CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE categories (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    parent_id   UUID REFERENCES categories(id) ON DELETE SET NULL,
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(255) NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_categories_parent_id ON categories(parent_id);
CREATE INDEX idx_categories_slug ON categories(slug);

CREATE TABLE orders (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id         UUID NOT NULL,
    accepted_offer_id   UUID,
    category_id         UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    status              VARCHAR(50) NOT NULL DEFAULT 'created',
    price               DECIMAL(12, 2) NOT NULL,
    final_price         DECIMAL(12, 2),
    currency            VARCHAR(10) NOT NULL DEFAULT 'RUB',
    address             TEXT NOT NULL,
    latitude            DOUBLE PRECISION,
    longitude           DOUBLE PRECISION,
    title               VARCHAR(500) NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ
);

CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_category_id ON orders(category_id);
CREATE INDEX idx_orders_accepted_offer_id ON orders(accepted_offer_id);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);

CREATE TABLE order_status_history (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id    UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    old_status  VARCHAR(50),
    new_status  VARCHAR(50) NOT NULL,
    changed_by  UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_status_history_order_id ON order_status_history(order_id);
CREATE INDEX idx_order_status_history_created_at ON order_status_history(created_at DESC);

CREATE TABLE reviews (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id    UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    from_user_id UUID NOT NULL,
    to_user_id  UUID NOT NULL,
    rating      SMALLINT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_reviews_order_from_user ON reviews(order_id, from_user_id);
CREATE INDEX idx_reviews_to_user_id ON reviews(to_user_id);
CREATE INDEX idx_reviews_from_user_id ON reviews(from_user_id);

CREATE TABLE complaints (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reporter_user_id UUID NOT NULL,
    target_user_id  UUID NOT NULL,
    order_id        UUID REFERENCES orders(id) ON DELETE SET NULL,
    message         TEXT NOT NULL,
    status          VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_complaints_status ON complaints(status);
CREATE INDEX idx_complaints_reporter ON complaints(reporter_user_id);
CREATE INDEX idx_complaints_target ON complaints(target_user_id);
