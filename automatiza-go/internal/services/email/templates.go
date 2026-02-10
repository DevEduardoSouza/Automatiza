package email

// Email templates in HTML format

const baseStyle = `
<style>
    body {
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
        line-height: 1.6;
        color: #333;
        max-width: 600px;
        margin: 0 auto;
        padding: 20px;
    }
    .container {
        background: #ffffff;
        border-radius: 8px;
        padding: 30px;
        box-shadow: 0 2px 10px rgba(0,0,0,0.1);
    }
    .header {
        text-align: center;
        margin-bottom: 30px;
    }
    .logo {
        font-size: 28px;
        font-weight: bold;
        color: #4F46E5;
    }
    h1 {
        color: #1a1a1a;
        margin-bottom: 20px;
    }
    .button {
        display: inline-block;
        background: #4F46E5;
        color: #ffffff !important;
        text-decoration: none;
        padding: 12px 24px;
        border-radius: 6px;
        font-weight: 500;
        margin: 20px 0;
    }
    .button:hover {
        background: #4338CA;
    }
    .highlight {
        background: #F3F4F6;
        padding: 15px;
        border-radius: 6px;
        font-family: monospace;
        font-size: 18px;
        text-align: center;
        margin: 20px 0;
    }
    .footer {
        margin-top: 30px;
        padding-top: 20px;
        border-top: 1px solid #e5e7eb;
        font-size: 12px;
        color: #6b7280;
        text-align: center;
    }
    .warning {
        background: #FEF3C7;
        border-left: 4px solid #F59E0B;
        padding: 15px;
        margin: 20px 0;
    }
    .success {
        background: #D1FAE5;
        border-left: 4px solid #10B981;
        padding: 15px;
        margin: 20px 0;
    }
</style>
`

const welcomeTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    ` + baseStyle + `
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Automatiza</div>
        </div>

        <h1>Bem-vindo, {{.Name}}!</h1>

        <p>Sua conta foi criada com sucesso no Automatiza. Estamos muito felizes em te-lo conosco!</p>

        <div class="success">
            <strong>Conta criada com sucesso!</strong>
        </div>

        <p>Aqui estao suas credenciais de acesso:</p>

        <p><strong>Senha temporaria:</strong></p>
        <div class="highlight">{{.Password}}</div>

        <p><strong>Importante:</strong> Por seguranca, recomendamos que voce altere sua senha apos o primeiro login.</p>

        <p style="text-align: center;">
            <a href="{{if .LoginURL}}{{.LoginURL}}{{else}}https://astecwms.com.br/auth{{end}}" class="button">Acessar Minha Conta</a>
        </p>

        <h2>Proximos passos:</h2>
        <ol>
            <li>Faca login na plataforma</li>
            <li>Conecte sua primeira instancia do WhatsApp</li>
            <li>Configure suas automacoes</li>
            <li>Comece a enviar ofertas automaticamente!</li>
        </ol>

        <p>Se precisar de ajuda, nossa equipe de suporte esta disponivel para ajuda-lo.</p>

        <div class="footer">
            <p>&copy; {{.Year}} Automatiza. Todos os direitos reservados.</p>
            <p>Este e-mail foi enviado porque voce criou uma conta no Automatiza.</p>
        </div>
    </div>
</body>
</html>
`

const passwordResetTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    ` + baseStyle + `
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Automatiza</div>
        </div>

        <h1>Redefinir Senha</h1>

        <p>Ola{{if .Name}}, {{.Name}}{{end}}!</p>

        <p>Recebemos uma solicitacao para redefinir a senha da sua conta no Automatiza.</p>

        <p>Clique no botao abaixo para criar uma nova senha:</p>

        <p style="text-align: center;">
            <a href="{{.ResetLink}}" class="button">Redefinir Minha Senha</a>
        </p>

        <div class="warning">
            <strong>Atencao:</strong> Este link expira em 1 hora por motivos de seguranca.
        </div>

        <p>Se voce nao solicitou a redefinicao de senha, pode ignorar este e-mail com seguranca. Sua senha permanecera inalterada.</p>

        <p>Se o botao nao funcionar, copie e cole o link abaixo no seu navegador:</p>
        <p style="word-break: break-all; font-size: 12px; color: #6b7280;">{{.ResetLink}}</p>

        <div class="footer">
            <p>&copy; {{.Year}} Automatiza. Todos os direitos reservados.</p>
            <p>Este e-mail foi enviado porque alguem solicitou a redefinicao de senha para esta conta.</p>
        </div>
    </div>
</body>
</html>
`

const planUpgradeTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    ` + baseStyle + `
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Automatiza</div>
        </div>

        <h1>Parabens! Plano Atualizado</h1>

        <p>Ola, {{.Name}}!</p>

        <div class="success">
            <strong>Seu plano foi atualizado para {{.Plan}} com sucesso!</strong>
        </div>

        <p>Agora voce tem acesso a todos os recursos do plano {{.Plan}}:</p>

        {{if eq .PlanCode "pro"}}
        <ul>
            <li>Ate 10 automacoes simultaneas</li>
            <li>Integracao com Shopee e Mercado Livre</li>
            <li>Monitoramento de grupos</li>
            <li>Suporte prioritario</li>
        </ul>
        {{else if eq .PlanCode "elite"}}
        <ul>
            <li>Automacoes ilimitadas</li>
            <li>Todas as integracoes disponiveis</li>
            <li>Monitoramento avancado com IA</li>
            <li>Load balancing entre servidores</li>
            <li>Suporte VIP 24/7</li>
            <li>API de acesso</li>
        </ul>
        {{end}}

        <p style="text-align: center;">
            <a href="https://astecwms.com.br/dashboard" class="button">Acessar Dashboard</a>
        </p>

        <p>Obrigado por confiar no Automatiza!</p>

        <div class="footer">
            <p>&copy; {{.Year}} Automatiza. Todos os direitos reservados.</p>
            <p>Duvidas sobre seu plano? Entre em contato com nosso suporte.</p>
        </div>
    </div>
</body>
</html>
`

const suspensionWarningTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    ` + baseStyle + `
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">Automatiza</div>
        </div>

        <h1>Aviso Importante</h1>

        <p>Ola, {{.Name}}!</p>

        <div class="warning">
            <strong>Atencao:</strong> Identificamos um problema com seu pagamento.
        </div>

        <p>Sua conta sera suspensa em <strong>{{.Days}} dias</strong> se o problema nao for resolvido.</p>

        <p>Quando sua conta for suspensa:</p>
        <ul>
            <li>Suas automacoes serao pausadas</li>
            <li>Voce nao podera enviar mensagens</li>
            <li>Seus dados serao mantidos por 30 dias</li>
        </ul>

        <p>Para evitar a suspensao, por favor regularize seu pagamento:</p>

        <p style="text-align: center;">
            <a href="https://astecwms.com.br/billing" class="button">Regularizar Pagamento</a>
        </p>

        <p>Se voce acredita que isso e um erro ou ja realizou o pagamento, entre em contato com nosso suporte.</p>

        <div class="footer">
            <p>&copy; {{.Year}} Automatiza. Todos os direitos reservados.</p>
            <p>Este e-mail foi enviado devido a uma pendencia de pagamento.</p>
        </div>
    </div>
</body>
</html>
`
